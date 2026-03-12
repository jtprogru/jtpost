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
	"github.com/jtprogru/jtpost/internal/logger"
	"github.com/spf13/cobra"
)

var (
	serveAddr   string = "0.0.0.0"
	servePort   int    = 8080
	serveVerbose bool  = false
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Запустить HTTP сервер",
	Long:  `Запускает встроенный HTTP сервер с REST API и Web UI для управления постами.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Создаём логгер
		logCfg := logger.Config{
			Output: os.Stdout,
			Debug:  serveVerbose,
			Prefix: "[HTTP]",
		}
		log := logger.New(logCfg)

		if serveVerbose {
			log.Info("Verbose режим включён (DEBUG уровень)")
		}

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
			log.Info("✅ Telegram publisher инициализирован")
		} else {
			log.Warn("⚠️  Telegram publisher не настроен (отсутствует конфигурация)")
		}

		// Создаём HTTP сервер с логгером
		serverCfg := httpapi.ServerConfig{
			Service:    service,
			Publishers: publishers,
			Logger:     log,
		}
		server := httpapi.NewServerWithConfig(serverCfg)

		// Оборачиваем сервер в middleware
		handler := httpapi.LoggingMiddleware(log, server)
		handler = httpapi.RecoveryMiddleware(log, handler)

		// Настраиваем адрес
		addr := fmt.Sprintf("%s:%d", serveAddr, servePort)

		// Создаём HTTP сервер с таймаутами
		httpServer := &http.Server{
			Addr:         addr,
			Handler:      handler,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		}

		// Канал для сигналов завершения
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

		// Запускаем сервер в горутине
		go func() {
			log.Info("🚀 jtpost HTTP сервер запущен на http://%s", addr)
			log.Info("📊 Web UI доступен в браузере")
			log.Info("🔌 API endpoints:")
			log.Info("   GET    /api/posts      — список постов")
			log.Info("   GET    /api/posts/{id} — получить пост")
			log.Info("   PATCH  /api/posts/{id} — обновить пост")
			log.Info("   DELETE /api/posts/{id} — удалить пост")
			log.Info("   POST   /api/posts      — создать пост")
			log.Info("   POST   /api/posts/{id}/publish — опубликовать пост")
			log.Info("   GET    /api/stats      — статистика")
			log.Info("   GET    /api/next       — рекомендация")
			log.Info("   GET    /api/plan       — план публикаций")
			log.Info("")
			log.Info("Нажмите Ctrl+C для остановки")

			if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Error("❌ Ошибка сервера: %v", err)
				os.Exit(1)
			}
		}()

		// Ждём сигнал завершения
		<-quit
		log.Info("\n🛑 Остановка сервера...")

		// Graceful shutdown с таймаутом
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(ctx); err != nil {
			return fmt.Errorf("ошибка остановки сервера: %w", err)
		}

		log.Info("✅ Сервер успешно остановлен")
		return nil
	},
}

func init() {
	serveCmd.Flags().StringVarP(&serveAddr, "addr", "a", "localhost", "адрес для прослушивания")
	serveCmd.Flags().IntVarP(&servePort, "port", "p", 8080, "порт для прослушивания")
	serveCmd.Flags().BoolVarP(&serveVerbose, "verbose", "v", false, "включить подробное логирование (DEBUG режим)")
}
