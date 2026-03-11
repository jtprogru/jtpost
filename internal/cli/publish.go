package cli

import (
	"context"
	"fmt"

	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/adapters/fsrepo"
	"github.com/jtprogru/jtpost/internal/adapters/telegram"
	"github.com/jtprogru/jtpost/internal/adapters/telegramconv"
	"github.com/jtprogru/jtpost/internal/core"
	"github.com/spf13/cobra"
)

var (
	publishTo     string
	publishDryRun bool
)

var publishCmd = &cobra.Command{
	Use:   "publish <id>",
	Short: "Опубликовать пост",
	Long:  `Публикует пост в Telegram.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := core.PostID(args[0])

		// Загружаем конфигурацию
		configPath, _ := cmd.Flags().GetString("config")
		cfg, err := loadConfigOrCreateDefault(configPath)
		if err != nil {
			return err
		}

		// Создаём репозиторий
		repo, err := fsrepo.NewFileSystemRepository(cfg.PostsDir)
		if err != nil {
			return fmt.Errorf("ошибка создания репозитория: %w", err)
		}

		// Создаём сервис
		service := core.NewPostService(repo, core.SystemClock{})

		// Получаем пост
		post, err := service.GetByID(cmd.Context(), id)
		if err != nil {
			return fmt.Errorf("ошибка получения поста: %w", err)
		}

		// Проверяем готовность к публикации
		if post.Status != core.StatusReady && post.Status != core.StatusScheduled {
			return fmt.Errorf("%w: статус поста '%s' не позволяет публикацию (требуется ready или scheduled)", core.ErrNotReadyToPublish, post.Status)
		}

		// Публикация в Telegram
		if publishTo == "telegram" {
			return publishToTelegram(cmd.Context(), post, cfg, service)
		}

		return fmt.Errorf("неподдерживаемая платформа: %s", publishTo)
	},
}

func init() {
	publishCmd.Flags().StringVarP(&publishTo, "to", "t", "telegram", "платформа для публикации (telegram)")
	publishCmd.Flags().BoolVarP(&publishDryRun, "dry-run", "d", false, "режим предпросмотра без публикации")
	_ = publishCmd.MarkFlagRequired("to")
}

// publishToTelegram публикует пост в Telegram.
func publishToTelegram(ctx context.Context, post *core.Post, cfg *config.Config, service *core.PostService) error {
	// Проверяем конфигурацию Telegram
	if cfg.Telegram.BotToken == "" || cfg.Telegram.ChatID == "" {
		return fmt.Errorf("не настроен Telegram: укажите bot_token и chat_id в .jtpost.yaml или env переменных")
	}

	// Создаём publisher
	tgCfg := telegram.Config{
		BotToken:  cfg.Telegram.BotToken,
		ChannelID: cfg.Telegram.ChatID,
	}
	publisher := telegram.NewPublisher(tgCfg)

	// Режим dry-run — показываем предпросмотр
	if publishDryRun {
		return showDryRunPreview(post, tgCfg.ChannelID)
	}

	// Публикуем
	updatedPost, err := publisher.Publish(ctx, post)
	if err != nil {
		return fmt.Errorf("ошибка публикации в Telegram: %w", err)
	}

	// Сохраняем обновлённый пост
	if err := service.UpdatePost(ctx, updatedPost); err != nil {
		return fmt.Errorf("ошибка сохранения поста: %w", err)
	}

	fmt.Printf("✅ Пост опубликован в Telegram\n")
	fmt.Printf("📝 Заголовок: %s\n", post.Title)
	fmt.Printf("🔗 Ссылка: %s\n", updatedPost.External.TelegramURL)

	return nil
}

// showDryRunPreview показывает предпросмотр сообщения без публикации.
func showDryRunPreview(post *core.Post, channelID string) error {
	htmlContent := telegramconv.MarkdownToHTML(post.Content)
	messageText := fmt.Sprintf("<b>%s</b>\n\n%s", telegramconv.EscapeHTML(post.Title), htmlContent)

	fmt.Println("🔍 Предпросмотр публикации в Telegram")
	fmt.Println("=====================================")
	fmt.Printf("📍 Канал: %s\n", channelID)
	fmt.Printf("📝 Заголовок: %s\n", post.Title)
	fmt.Printf("📏 Длина сообщения: %d символов\n", len(messageText))
	fmt.Println()
	fmt.Println("📄 Текст сообщения:")
	fmt.Println("-------------------------------------")
	fmt.Println(messageText)
	fmt.Println("-------------------------------------")
	fmt.Println()
	fmt.Println("⚠️  Это режим предпросмотра. Для реальной публикации уберите флаг --dry-run")

	return nil
}
