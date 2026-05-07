package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jtprogru/jtpost/internal/adapters/httpapi"
	"github.com/jtprogru/jtpost/internal/adapters/storage"
	"github.com/jtprogru/jtpost/internal/adapters/telegram"
	"github.com/jtprogru/jtpost/internal/adapters/webui"
	"github.com/jtprogru/jtpost/internal/core"
	"github.com/jtprogru/jtpost/internal/logger"
	"github.com/spf13/cobra"
)

var (
	serveAddr      string = "0.0.0.0"
	servePort      int    = 8080
	serveVerbose   bool   = false
	serveLogFormat string = "text"
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
			Prefix: "http",
			Format: logger.ParseFormat(serveLogFormat),
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

		// Создаём Bundle (включает Posts/Users/Tokens)
		bundle, err := storage.OpenBundle(cfg)
		if err != nil {
			return fmt.Errorf("ошибка создания репозитория: %w", err)
		}
		defer bundle.Closer.Close()

		// Создаём сервис
		service := core.NewPostService(bundle.Posts, core.SystemClock{})

		// Создаём publisher
		var publisher core.Publisher

		// Telegram publisher
		if cfg.Telegram.BotToken != "" && cfg.Telegram.ChatID != "" {
			publisher = telegram.NewPublisher(telegram.Config{
				BotToken:    cfg.Telegram.BotToken,
				ChannelID:   cfg.Telegram.ChatID,
				SiteBaseURL: cfg.Server.BaseURL,
				UploadDir:   cfg.Server.Upload.Dir,
				UploadRoute: "/ui/uploads/",
			})
			log.Info("✅ Telegram publisher инициализирован")
		} else {
			log.Warn("⚠️  Telegram publisher не настроен (отсутствует конфигурация)")
		}

		// Создаём AuthService и OAuthService (nil если auth.type != token)
		var authSvc *core.AuthService
		var oauthSvc *core.OAuthService
		if cfg.Auth.Type == "token" {
			if bundle.Users == nil || bundle.Tokens == nil {
				return fmt.Errorf("auth.type=token requires sqlite or postgres storage")
			}
			authSvc = core.NewAuthService(bundle.Users, bundle.Tokens, bundle.Sessions, core.HasherFromConfig(cfg.Auth.PasswordHasher), core.SystemClock{})
			oauthSvc = buildOAuthService(cfg, bundle, log)
		}

		// AuditService — nil-safe wrapper над AuditRepository (nil для fs).
		var auditSvc *core.AuditService
		if bundle.AuditLog != nil {
			auditSvc = core.NewAuditService(bundle.AuditLog, core.SystemClock{})
		}

		// Web UI v2 (htmx + templ).
		bus := core.NewMemoryBus(32)
		// HistoryProvider — опциональный, реализован только в gitrepo.
		var history core.HistoryProvider
		if hp, ok := bundle.Posts.(core.HistoryProvider); ok {
			history = hp
		}
		ui := webui.NewHandler(webui.Config{
			Service:   service,
			Auth:      authSvc,
			Audit:     auditSvc,
			AuditRepo: bundle.AuditLog,
			Bus:       bus,
			History:   history,
			Cfg:       cfg,
			Logger:    log,
		})

		// Создаём HTTP сервер с логгером
		serverCfg := httpapi.ServerConfig{
			Service:      service,
			Publisher:    publisher,
			AuthService:  authSvc,
			OAuthService: oauthSvc,
			AuditService: auditSvc,
			AuditRepo:    bundle.AuditLog,
			Outbox:       bundle.Outbox,
			UI:           ui,
			Logger:       log,
			Config:       cfg,
		}
		server := httpapi.NewServerWithConfig(serverCfg)

		// Оборачиваем сервер в middleware-chain.
		var handler http.Handler = server
		if cfg.Auth.Type == "token" {
			// Bearer (soft) → Session (soft) → CSRF → RequireAuth (final 401).
			handler = httpapi.RequireAuthMiddleware()(handler)
			handler = httpapi.CSRFMiddleware()(handler)
			handler = httpapi.SessionMiddleware(authSvc)(handler)
			handler = httpapi.BearerTokenMiddleware(authSvc)(handler)
			log.Info("🔐 Auth chain: Bearer + Session + CSRF включён")
		} else {
			handler = httpapi.TenantFromConfigMiddleware(cfg)(handler)
		}
		// AuditContext (IP/UA) до auth, чтобы все handler'ы (включая
		// неаутентифицированный login) получили эти поля в ctx.
		handler = httpapi.AuditContextMiddleware(cfg.Server.RateLimit.TrustProxyHeader)(handler)
		// Rate limit идёт после auth, чтобы ключевание шло по User.ID при
		// аутентифицированных запросах (см. RateLimitMiddleware).
		// Применяется до LoggingMiddleware → отказы тоже логируются.
		rlStop := make(chan struct{})
		defer close(rlStop)
		if cfg.Server.RateLimit.Enabled {
			handler = httpapi.RateLimitMiddleware(httpapi.RateLimitConfig{
				Enabled:           true,
				RequestsPerMinute: cfg.Server.RateLimit.RequestsPerMinute,
				Burst:             cfg.Server.RateLimit.Burst,
				TrustProxyHeader:  cfg.Server.RateLimit.TrustProxyHeader,
			}, log, rlStop)(handler)
			log.Info("🚦 Rate limit: %d req/min (burst=%d, trust_proxy=%v)",
				cfg.Server.RateLimit.RequestsPerMinute, cfg.Server.RateLimit.Burst, cfg.Server.RateLimit.TrustProxyHeader)
		}
		handler = httpapi.LoggingMiddleware(log, handler)
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
	serveCmd.Flags().StringVar(&serveLogFormat, "log-format", "text", "формат логов: text (человекочитаемый) или json (структурированный)")
}
