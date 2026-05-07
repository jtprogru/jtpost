// Package telegram предоставляет адаптер для публикации постов в Telegram.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jtprogru/jtpost/internal/adapters/telegramconv"
	"github.com/jtprogru/jtpost/internal/core"
)

// Publisher реализует интерфейс core.Publisher для Telegram.
type Publisher struct {
	botToken    string
	channelID   string
	client      *http.Client
	baseURL     string
	siteURL     string // абсолютный URL сайта для resolve относительных путей картинок
	uploadDir   string // локальный путь к каталогу /ui/uploads/* для multipart-fallback
	uploadRoute string // url-prefix внутри которого считаем путь относящимся к uploadDir
}

// Config конфигурация Telegram Publisher.
type Config struct {
	BotToken  string `yaml:"bot_token"`
	ChannelID string `yaml:"channel_id"` // @channelname или -1001234567890
	// SiteBaseURL — абсолютный URL сервиса (server.base_url). Используется чтобы
	// markdown-image вида `/ui/uploads/...` превращать в публичный URL, который
	// Telegram сам скачает через sendPhoto.
	SiteBaseURL string `yaml:"site_base_url"`
	// UploadDir + UploadRoute — fallback для приватных uploads (когда SiteBaseURL
	// не задан или сайт недоступен Telegram'у). Файл читается локально и
	// заливается через multipart/form-data. Если UploadDir пуст — fallback не
	// активирован.
	UploadDir   string `yaml:"upload_dir"`
	UploadRoute string `yaml:"upload_route"` // обычно "/ui/uploads/"
}

// telegramCaptionLimit — лимит caption на одно фото.
const telegramCaptionLimit = 1024

// NewPublisher создаёт новый Telegram Publisher.
func NewPublisher(cfg Config) *Publisher {
	route := cfg.UploadRoute
	if route == "" && cfg.UploadDir != "" {
		route = "/ui/uploads/"
	}
	return &Publisher{
		botToken:    cfg.BotToken,
		channelID:   cfg.ChannelID,
		client:      &http.Client{Timeout: 60 * time.Second},
		baseURL:     "https://api.telegram.org/bot",
		siteURL:     strings.TrimRight(cfg.SiteBaseURL, "/"),
		uploadDir:   cfg.UploadDir,
		uploadRoute: route,
	}
}

// Publish публикует пост в Telegram канал.
func (p *Publisher) Publish(ctx context.Context, post *core.Post) (*core.Post, error) {
	if post.Content == "" {
		return nil, fmt.Errorf("%w: пустой контент поста", core.ErrValidation)
	}

	// Если есть картинки и сконфигурирован SiteBaseURL — отправляем фото
	// как primary message (с caption=title+content, если умещается).
	images := ExtractImages(post.Content)
	contentForCaption := StripImages(post.Content)
	htmlBody := telegramconv.MarkdownToHTML(contentForCaption)
	primaryHTML := fmt.Sprintf("<b>%s</b>\n\n%s", telegramconv.EscapeHTML(post.Title), htmlBody)

	var msgURL string
	var err error
	caption := primaryHTML
	var followup string
	if len(caption) > telegramCaptionLimit {
		caption = fmt.Sprintf("<b>%s</b>", telegramconv.EscapeHTML(post.Title))
		followup = htmlBody
	}
	sources := p.resolvePhotoSources(images)
	switch {
	case len(sources) >= 2:
		msgURL, err = p.sendMediaGroup(ctx, sources, caption)
	case len(sources) == 1 && sources[0].kind == photoSourceFile:
		msgURL, err = p.sendPhotoFile(ctx, sources[0].path, caption)
	case len(sources) == 1 && sources[0].kind == photoSourceURL:
		msgURL, err = p.sendPhoto(ctx, sources[0].path, caption)
	default:
		msgURL, err = p.sendMessage(ctx, primaryHTML)
		followup = ""
	}
	if err != nil {
		return nil, fmt.Errorf("ошибка отправки сообщения: %w", err)
	}
	if followup != "" {
		if _, ferr := p.sendMessage(ctx, followup); ferr != nil {
			return nil, fmt.Errorf("ошибка follow-up сообщения: %w", ferr)
		}
	}

	// Обновляем пост с ссылкой
	post.External.TelegramURL = msgURL
	post.PublishedAt = ptrTime(time.Now())

	return post, nil
}

// telegramMessage структура ответа Telegram API.
type telegramMessage struct {
	MessageID int `json:"message_id"`
	Chat      struct {
		ID       int64  `json:"id"`
		Username string `json:"username,omitempty"`
	} `json:"chat"`
}

// ptrTime возвращает указатель на time.Time.
func ptrTime(t time.Time) *time.Time {
	return &t
}

// ValidateConfig проверяет конфигурацию Telegram.
func ValidateConfig(cfg Config) error {
	if cfg.BotToken == "" {
		return fmt.Errorf("%w: bot_token не указан", core.ErrValidation)
	}
	if cfg.ChannelID == "" {
		return fmt.Errorf("%w: channel_id не указан", core.ErrValidation)
	}
	return nil
}

// BotInfo описывает учётную запись бота, возвращаемую методом getMe.
type BotInfo struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

// GetMe возвращает информацию о боте, проверяя валидность токена.
func (p *Publisher) GetMe(ctx context.Context) (*BotInfo, error) {
	apiURL := fmt.Sprintf("%s%s/getMe", p.baseURL, p.botToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API вернул статус: %d", resp.StatusCode)
	}

	var result struct {
		OK     bool    `json:"ok"`
		Result BotInfo `json:"result"`
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if !result.OK {
		return nil, errors.New("бот не авторизован")
	}

	return &result.Result, nil
}

// photoSourceKind — тип резолва для image-URL.
type photoSourceKind int

const (
	photoSourceNone photoSourceKind = iota
	photoSourceURL                  // path = публичный URL для sendPhoto
	photoSourceFile                 // path = локальный файл для multipart upload
)

type photoSource struct {
	kind photoSourceKind
	path string
}

// telegramMediaGroupMax — лимит Telegram на sendMediaGroup.
const telegramMediaGroupMax = 10

// resolvePhotoSources резолвит все картинки до источников (URL или файл),
// подходящих для отправки в Telegram. Лимит — telegramMediaGroupMax (первые
// разрешимые). Картинки, которые невозможно разрешить (relative без siteURL и
// без UploadDir), пропускаются.
func (p *Publisher) resolvePhotoSources(imgs []MDImage) []photoSource {
	out := make([]photoSource, 0, len(imgs))
	for _, im := range imgs {
		if len(out) >= telegramMediaGroupMax {
			break
		}
		s := p.resolveOne(im)
		if s.kind != photoSourceNone {
			out = append(out, s)
		}
	}
	return out
}

// resolveOne — одна картинка → photoSource (тот же приоритет, что и у
// firstPhotoSource: absolute URL → multipart-file → siteURL-based).
func (p *Publisher) resolveOne(im MDImage) photoSource {
	if strings.HasPrefix(im.URL, "https://") || strings.HasPrefix(im.URL, "http://") {
		return photoSource{photoSourceURL, im.URL}
	}
	if !strings.HasPrefix(im.URL, "/") {
		return photoSource{photoSourceNone, ""}
	}
	if p.uploadDir != "" && p.uploadRoute != "" && strings.HasPrefix(im.URL, p.uploadRoute) {
		rel := strings.TrimPrefix(im.URL, p.uploadRoute)
		if rel != "" && !strings.Contains(rel, "..") {
			abs, err := filepath.Abs(filepath.Join(p.uploadDir, filepath.FromSlash(rel)))
			if err == nil {
				rootAbs, rerr := filepath.Abs(p.uploadDir)
				if rerr == nil && strings.HasPrefix(abs, rootAbs+string(filepath.Separator)) {
					if _, statErr := os.Stat(abs); statErr == nil {
						return photoSource{photoSourceFile, abs}
					}
				}
			}
		}
	}
	if p.siteURL != "" {
		return photoSource{photoSourceURL, p.siteURL + im.URL}
	}
	return photoSource{photoSourceNone, ""}
}

// firstPhotoSource — первый разрешимый источник (для одиночного sendPhoto).
func (p *Publisher) firstPhotoSource(imgs []MDImage) photoSource {
	for _, im := range imgs {
		if s := p.resolveOne(im); s.kind != photoSourceNone {
			return s
		}
	}
	return photoSource{photoSourceNone, ""}
}

// firstResolvableImage — backward-compat shortcut. Возвращает только URL-вариант
// (используется в тестах). Для нового кода — firstPhotoSource.
func (p *Publisher) firstResolvableImage(imgs []MDImage) string {
	if s := p.firstPhotoSource(imgs); s.kind == photoSourceURL {
		return s.path
	}
	return ""
}

// sendMediaGroup публикует 2..10 фото одной "галереей". Caption применяется к
// первому элементу (как делает Telegram-клиент). Источники могут смешиваться:
// URL-based и file-based; файлы заливаются как `attach://N`-references в одном
// multipart-запросе. Возвращает URL первого сообщения группы.
func (p *Publisher) sendMediaGroup(ctx context.Context, sources []photoSource, caption string) (string, error) {
	if len(sources) < 2 {
		return "", errors.New("media group требует минимум 2 элемента")
	}
	if len(sources) > telegramMediaGroupMax {
		sources = sources[:telegramMediaGroupMax]
	}
	type inputMediaPhoto struct {
		Type      string `json:"type"`
		Media     string `json:"media"`
		Caption   string `json:"caption,omitempty"`
		ParseMode string `json:"parse_mode,omitempty"`
	}
	media := make([]inputMediaPhoto, len(sources))
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	_ = mw.WriteField("chat_id", p.channelID)

	for i, s := range sources {
		item := inputMediaPhoto{Type: "photo"}
		if i == 0 && caption != "" {
			item.Caption = caption
			item.ParseMode = "HTML"
		}
		switch s.kind {
		case photoSourceURL:
			item.Media = s.path
		case photoSourceFile:
			attachName := fmt.Sprintf("photo%d", i)
			item.Media = "attach://" + attachName
			f, err := os.Open(s.path)
			if err != nil {
				return "", fmt.Errorf("открытие %s: %w", s.path, err)
			}
			fw, ferr := mw.CreateFormFile(attachName, filepath.Base(s.path))
			if ferr != nil {
				_ = f.Close()
				return "", fmt.Errorf("multipart создание поля: %w", ferr)
			}
			if _, cerr := io.Copy(fw, f); cerr != nil {
				_ = f.Close()
				return "", fmt.Errorf("multipart копирование: %w", cerr)
			}
			_ = f.Close()
		default:
			return "", fmt.Errorf("неподдерживаемый source kind=%d", s.kind)
		}
		media[i] = item
	}
	mediaJSON, err := json.Marshal(media)
	if err != nil {
		return "", fmt.Errorf("сериализация media: %w", err)
	}
	_ = mw.WriteField("media", string(mediaJSON))
	if err := mw.Close(); err != nil {
		return "", fmt.Errorf("multipart close: %w", err)
	}

	apiURL := fmt.Sprintf("%s%s/sendMediaGroup", p.baseURL, p.botToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, body)
	if err != nil {
		return "", fmt.Errorf("создание запроса: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP запрос: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("чтение ответа: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API вернул ошибку: %s", string(respBody))
	}
	var result struct {
		OK     bool              `json:"ok"`
		Result []telegramMessage `json:"result"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("десериализация ответа: %w", err)
	}
	if !result.OK || len(result.Result) == 0 {
		return "", errors.New("API вернул ok=false или пустой массив")
	}
	channelName := strings.TrimPrefix(p.channelID, "-100")
	return fmt.Sprintf("https://t.me/%s/%d", channelName, result.Result[0].MessageID), nil
}

// sendPhotoFile отправляет фото из локального файла через multipart/form-data.
func (p *Publisher) sendPhotoFile(ctx context.Context, filePath, caption string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("открытие файла: %w", err)
	}
	defer func() { _ = f.Close() }()

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	_ = mw.WriteField("chat_id", p.channelID)
	if caption != "" {
		_ = mw.WriteField("caption", caption)
		_ = mw.WriteField("parse_mode", "HTML")
	}
	fw, err := mw.CreateFormFile("photo", filepath.Base(filePath))
	if err != nil {
		return "", fmt.Errorf("multipart создание поля: %w", err)
	}
	if _, err := io.Copy(fw, f); err != nil {
		return "", fmt.Errorf("multipart копирование: %w", err)
	}
	if err := mw.Close(); err != nil {
		return "", fmt.Errorf("multipart close: %w", err)
	}

	apiURL := fmt.Sprintf("%s%s/sendPhoto", p.baseURL, p.botToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, body)
	if err != nil {
		return "", fmt.Errorf("создание запроса: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP запрос: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("чтение ответа: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API вернул ошибку: %s", string(respBody))
	}
	var result struct {
		OK     bool            `json:"ok"`
		Result telegramMessage `json:"result"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("десериализация ответа: %w", err)
	}
	if !result.OK {
		return "", errors.New("API вернул ok=false")
	}
	channelName := strings.TrimPrefix(p.channelID, "-100")
	return fmt.Sprintf("https://t.me/%s/%d", channelName, result.Result.MessageID), nil
}

// sendPhoto публикует фото с опциональным caption. Возвращает URL поста.
func (p *Publisher) sendPhoto(ctx context.Context, photoURL, caption string) (string, error) {
	apiURL := fmt.Sprintf("%s%s/sendPhoto", p.baseURL, p.botToken)
	payload := map[string]any{
		"chat_id":    p.channelID,
		"photo":      photoURL,
		"caption":    caption,
		"parse_mode": "HTML",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("ошибка сериализации: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("ошибка создания запроса: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ошибка HTTP запроса: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ошибка чтения ответа: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API вернул ошибку: %s", string(respBody))
	}
	var result struct {
		OK     bool            `json:"ok"`
		Result telegramMessage `json:"result"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("ошибка десериализации ответа: %w", err)
	}
	if !result.OK {
		return "", errors.New("API вернул ok=false")
	}
	channelName := strings.TrimPrefix(p.channelID, "-100")
	return fmt.Sprintf("https://t.me/%s/%d", channelName, result.Result.MessageID), nil
}

// sendMessage отправляет текстовое сообщение в канал.
func (p *Publisher) sendMessage(ctx context.Context, text string) (string, error) {
	apiURL := fmt.Sprintf("%s%s/sendMessage", p.baseURL, p.botToken)

	payload := map[string]any{
		"chat_id":                  p.channelID,
		"text":                     text,
		"parse_mode":               "HTML",
		"disable_web_page_preview": true,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("ошибка сериализации: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("ошибка создания запроса: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ошибка HTTP запроса: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ошибка чтения ответа: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API вернул ошибку: %s", string(respBody))
	}

	var result struct {
		OK     bool            `json:"ok"`
		Result telegramMessage `json:"result"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("ошибка десериализации ответа: %w", err)
	}

	if !result.OK {
		return "", errors.New("API вернул ok=false")
	}

	// Формируем ссылку на сообщение
	// Для каналов: https://t.me/channelname/message_id
	channelName := strings.TrimPrefix(p.channelID, "-100")
	return fmt.Sprintf("https://t.me/%s/%d", channelName, result.Result.MessageID), nil
}
