// Package telegram предоставляет адаптер для публикации постов в Telegram.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jtprogru/jtpost/internal/adapters/telegramconv"
	"github.com/jtprogru/jtpost/internal/core"
)

// Publisher реализует интерфейс core.Publisher для Telegram.
type Publisher struct {
	botToken  string
	channelID string
	client    *http.Client
	baseURL   string
	siteURL   string // абсолютный URL сайта для resolve относительных путей картинок
}

// Config конфигурация Telegram Publisher.
type Config struct {
	BotToken  string `yaml:"bot_token"`
	ChannelID string `yaml:"channel_id"` // @channelname или -1001234567890
	// SiteBaseURL — абсолютный URL сервиса (server.base_url). Используется чтобы
	// markdown-image вида `/ui/uploads/...` превращать в публичный URL, который
	// Telegram сам скачает через sendPhoto. Если пуст — медиа не отправляется.
	SiteBaseURL string `yaml:"site_base_url"`
}

// telegramCaptionLimit — лимит caption на одно фото.
const telegramCaptionLimit = 1024

// NewPublisher создаёт новый Telegram Publisher.
func NewPublisher(cfg Config) *Publisher {
	return &Publisher{
		botToken:  cfg.BotToken,
		channelID: cfg.ChannelID,
		client:    &http.Client{Timeout: 30 * time.Second},
		baseURL:   "https://api.telegram.org/bot",
		siteURL:   strings.TrimRight(cfg.SiteBaseURL, "/"),
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
	if photoURL := p.firstResolvableImage(images); photoURL != "" {
		caption := primaryHTML
		var followup string
		if len(caption) > telegramCaptionLimit {
			// Caption — title-only; полный body уйдёт отдельным сообщением.
			caption = fmt.Sprintf("<b>%s</b>", telegramconv.EscapeHTML(post.Title))
			followup = htmlBody
		}
		msgURL, err = p.sendPhoto(ctx, photoURL, caption)
		if err != nil {
			return nil, fmt.Errorf("ошибка отправки фото: %w", err)
		}
		if followup != "" {
			if _, ferr := p.sendMessage(ctx, followup); ferr != nil {
				// Фото уже отправлено — лишь логируем followup-провал в ошибке.
				return nil, fmt.Errorf("ошибка follow-up сообщения: %w", ferr)
			}
		}
	} else {
		msgURL, err = p.sendMessage(ctx, primaryHTML)
		if err != nil {
			return nil, fmt.Errorf("ошибка отправки сообщения: %w", err)
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

// firstResolvableImage возвращает первый image URL, который Telegram сможет
// fetch'ить — абсолютный (https://...) или относительный, разрешённый через
// p.siteURL. Если siteURL не задан и URL относительный — возвращается "".
func (p *Publisher) firstResolvableImage(imgs []MDImage) string {
	for _, im := range imgs {
		if strings.HasPrefix(im.URL, "https://") || strings.HasPrefix(im.URL, "http://") {
			return im.URL
		}
		if p.siteURL == "" || !strings.HasPrefix(im.URL, "/") {
			continue
		}
		return p.siteURL + im.URL
	}
	return ""
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
