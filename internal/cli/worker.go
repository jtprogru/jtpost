package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jtprogru/jtpost/internal/adapters/storage"
	"github.com/jtprogru/jtpost/internal/adapters/telegram"
	"github.com/jtprogru/jtpost/internal/core"
	"github.com/jtprogru/jtpost/internal/logger"
	"github.com/spf13/cobra"
)

var (
	workerInterval    time.Duration
	workerMaxAttempts int
	workerLogFormat   string
	workerVerbose     bool
)

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Background worker для outbox-очереди публикаций",
}

var workerRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Запустить worker (long-running)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		cfg, err := loadConfigOrCreateDefault(configPath)
		if err != nil {
			return err
		}
		bundle, err := storage.OpenBundle(cfg)
		if err != nil {
			return fmt.Errorf("open storage: %w", err)
		}
		defer bundle.Closer.Close()
		if bundle.Outbox == nil {
			return fmt.Errorf("storage backend %q не поддерживает outbox (используйте sqlite или postgres)", cfg.Storage.Type)
		}
		if cfg.Telegram.BotToken == "" || cfg.Telegram.ChatID == "" {
			return fmt.Errorf("telegram bot_token и chat_id обязательны для worker")
		}
		pub := telegram.NewPublisher(telegram.Config{BotToken: cfg.Telegram.BotToken, ChannelID: cfg.Telegram.ChatID, SiteBaseURL: cfg.Server.BaseURL, UploadDir: cfg.Server.Upload.Dir, UploadRoute: "/ui/uploads/"})
		log := logger.New(logger.Config{
			Output: os.Stdout,
			Debug:  workerVerbose,
			Prefix: "worker",
			Format: logger.ParseFormat(workerLogFormat),
		})
		w := core.NewWorker(bundle.Outbox, bundle.Posts, pub, core.SystemClock{}, core.WorkerConfig{
			PollInterval: workerInterval,
			MaxAttempts:  workerMaxAttempts,
			Logger:       log,
		})

		ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		fmt.Printf("🚀 Worker запущен (poll=%s, max_attempts=%d). Ctrl+C для остановки.\n", workerInterval, workerMaxAttempts)
		if err := w.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	},
}

func init() {
	workerRunCmd.Flags().DurationVar(&workerInterval, "interval", 10*time.Second, "интервал опроса очереди")
	workerRunCmd.Flags().IntVar(&workerMaxAttempts, "max-attempts", 5, "максимальное число попыток до permanent fail")
	workerRunCmd.Flags().StringVar(&workerLogFormat, "log-format", "text", "формат логов: text или json")
	workerRunCmd.Flags().BoolVarP(&workerVerbose, "verbose", "v", false, "verbose режим (DEBUG уровень)")
	workerCmd.AddCommand(workerRunCmd)
}
