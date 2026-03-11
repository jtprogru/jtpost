package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jtprogru/jtpost/internal/adapters/fsrepo"
	"github.com/jtprogru/jtpost/internal/adapters/httpapi"
	"github.com/jtprogru/jtpost/internal/adapters/telegram"
	"github.com/jtprogru/jtpost/internal/core"
	"github.com/spf13/cobra"
)

var (
	serveAddr string = "0.0.0.0"
	servePort int = 8080
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Запустить HTTP сервер",
	Long:  `Запускает встроенный HTTP сервер с REST API и Web UI для управления постами.`,
	RunE: func(cmd *cobra.Command, args []string) error {
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

		// Создаём publishers
		publishers := make(map[core.Platform]core.Publisher)

		// Telegram publisher
		if cfg.Telegram.BotToken != "" && cfg.Telegram.ChatID != "" {
			tgPublisher := telegram.NewPublisher(telegram.Config{
				BotToken:  cfg.Telegram.BotToken,
				ChannelID: cfg.Telegram.ChatID,
			})
			publishers[core.PlatformTelegram] = tgPublisher
			fmt.Println("✅ Telegram publisher инициализирован")
		} else {
			fmt.Println("⚠️  Telegram publisher не настроен (отсутствует конфигурация)")
		}

		// Создаём HTTP сервер
		server := httpapi.NewServer(service, publishers)

		// Настраиваем адрес
		addr := fmt.Sprintf("%s:%d", serveAddr, servePort)

		// Создаём HTTP сервер с таймаутами
		httpServer := &http.Server{
			Addr:         addr,
			Handler:      server,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		}

		// Канал для сигналов завершения
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

		// Запускаем сервер в горутине
		go func() {
			fmt.Printf("🚀 jtpost HTTP сервер запущен на http://%s\n", addr)
			fmt.Println("📊 Web UI доступен в браузере")
			fmt.Println("🔌 API endpoints:")
			fmt.Println("   GET    /api/posts      — список постов")
			fmt.Println("   GET    /api/posts/{id} — получить пост")
			fmt.Println("   PATCH  /api/posts/{id} — обновить пост")
			fmt.Println("   DELETE /api/posts/{id} — удалить пост")
			fmt.Println("   POST   /api/posts      — создать пост")
			fmt.Println("   POST   /api/posts/{id}/publish — опубликовать пост")
			fmt.Println("   GET    /api/stats      — статистика")
			fmt.Println("   GET    /api/next       — рекомендация")
			fmt.Println("   GET    /api/plan       — план публикаций")
			fmt.Println()
			fmt.Println("Нажмите Ctrl+C для остановки")

			if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				fmt.Fprintf(os.Stderr, "❌ Ошибка сервера: %v\n", err)
				os.Exit(1)
			}
		}()

		// Ждём сигнал завершения
		<-quit
		fmt.Println("\n🛑 Остановка сервера...")

		// Graceful shutdown с таймаутом
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(ctx); err != nil {
			return fmt.Errorf("ошибка остановки сервера: %w", err)
		}

		fmt.Println("✅ Сервер успешно остановлен")
		return nil
	},
}

func init() {
	serveCmd.Flags().StringVarP(&serveAddr, "addr", "a", "localhost", "адрес для прослушивания")
	serveCmd.Flags().IntVarP(&servePort, "port", "p", 8080, "порт для прослушивания")
}
