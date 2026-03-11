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
}

// Config конфигурация Telegram Publisher.
type Config struct {
	BotToken  string `yaml:"bot_token"`
	ChannelID string `yaml:"channel_id"` // @channelname или -1001234567890
}

// NewPublisher создаёт новый Telegram Publisher.
func NewPublisher(cfg Config) *Publisher {
	return &Publisher{
		botToken:  cfg.BotToken,
		channelID: cfg.ChannelID,
		client:    &http.Client{Timeout: 30 * time.Second},
		baseURL:   "https://api.telegram.org/bot",
	}
}

// Platform возвращает платформу telegram.
func (p *Publisher) Platform() core.Platform {
	return "telegram"
}

// Publish публикует пост в Telegram канал.
func (p *Publisher) Publish(ctx context.Context, post *core.Post) (*core.Post, error) {
	if post.Content == "" {
		return nil, fmt.Errorf("%w: пустой контент поста", core.ErrValidation)
	}

	// Конвертируем Markdown в Telegram HTML
	htmlContent := telegramconv.MarkdownToHTML(post.Content)

	// Формируем сообщение с заголовком
	messageText := fmt.Sprintf("<b>%s</b>\n\n%s", telegramconv.EscapeHTML(post.Title), htmlContent)

	// Отправляем сообщение
	msgURL, err := p.sendMessage(ctx, messageText)
	if err != nil {
		return nil, fmt.Errorf("ошибка отправки сообщения: %w", err)
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

// TestConnection проверяет соединение с Telegram API.
func (p *Publisher) TestConnection() error {
	apiURL := fmt.Sprintf("%s%s/getMe", p.baseURL, p.botToken)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, apiURL, nil)
	if err != nil {
		return fmt.Errorf("ошибка создания запроса: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("ошибка подключения: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API вернул статус: %d", resp.StatusCode)
	}

	var result struct {
		OK   bool `json:"ok"`
		Result struct {
			ID        int    `json:"id"`
			FirstName string `json:"first_name"`
			Username  string `json:"username"`
		} `json:"result"`
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return err
	}

	if !result.OK {
		return errors.New("бот не авторизован")
	}

	return nil
}

// sendMessage отправляет текстовое сообщение в канал.
func (p *Publisher) sendMessage(ctx context.Context, text string) (string, error) {
	apiURL := fmt.Sprintf("%s%s/sendMessage", p.baseURL, p.botToken)

	payload := map[string]any{
		"chat_id":    p.channelID,
		"text":       text,
		"parse_mode": "HTML",
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
		OK     bool          `json:"ok"`
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
