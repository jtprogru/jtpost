package cli

import (
	"fmt"
	"os"

	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/core"
	"github.com/spf13/cobra"
)

var (
	migrateFrom      string
	migrateTo        string
	migrateDryRun    bool
	migrateOverwrite bool
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Миграция данных постов между storage backend",
	Long: `Переносит все посты из source backend в target backend через core.MigratableRepository.

Поддерживаются: fs, sqlite, postgres. Параметры выбираются через --from и --to (обязательны).
Старый формат --db <path> больше не поддерживается; используйте storage.sqlite.dsn в конфиге.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Старый --db флаг — отдельная ошибка с подсказкой.
		if cmd.Flags().Changed("db") {
			fmt.Fprintln(os.Stderr, "flag --db is no longer supported, use --from/--to and storage.sqlite.dsn in config")
			os.Exit(2)
		}

		if migrateFrom == "" || migrateTo == "" {
			return fmt.Errorf("--from and --to are required")
		}
		if migrateFrom == migrateTo {
			return fmt.Errorf("source and target must differ")
		}
		if !isValidStorageType(migrateFrom) {
			return fmt.Errorf("invalid --from %q (must be fs|sqlite|postgres)", migrateFrom)
		}
		if !isValidStorageType(migrateTo) {
			return fmt.Errorf("invalid --to %q (must be fs|sqlite|postgres)", migrateTo)
		}

		ctx := cmd.Context()

		configPath, _ := cmd.Flags().GetString("config")
		cfg, err := loadConfigForMigrate(configPath)
		if err != nil {
			return err
		}

		fmt.Printf("📦 Миграция постов\n═══════════════════════════════════════\n")
		fmt.Printf("Источник: %s\nНазначение: %s\nТестовый запуск: %v\n\n", migrateFrom, migrateTo, migrateDryRun)

		srcRepo, srcCloser, err := openRepoAs(cfg, migrateFrom)
		if err != nil {
			return fmt.Errorf("ошибка открытия source (%s): %w", migrateFrom, err)
		}
		defer srcCloser.Close()

		ctx = scopeContext(ctx, cfg.Auth.TenantDefault, cfg.Auth.AuthorDefault)
		allPosts, err := srcRepo.List(ctx, core.PostFilter{TenantID: cfg.Auth.TenantDefault})
		if err != nil {
			return fmt.Errorf("ошибка получения списка постов: %w", err)
		}

		fmt.Printf("📊 Найдено постов: %d\n", len(allPosts))
		if len(allPosts) == 0 {
			fmt.Println("✅ Нечего мигрировать — посты не найдены")
			return nil
		}

		if migrateDryRun {
			fmt.Println("\n📋 Посты для миграции:")
			for _, post := range allPosts {
				fmt.Printf("  • %s (%s) — %s\n", post.Title, post.Slug, post.Status)
			}
			fmt.Println("\n⚠️  Это режим предпросмотра. Уберите --dry-run для реальной миграции")
			return nil
		}

		dstRepo, dstCloser, err := openRepoAs(cfg, migrateTo)
		if err != nil {
			return fmt.Errorf("ошибка открытия target (%s): %w", migrateTo, err)
		}
		defer dstCloser.Close()

		mig, ok := dstRepo.(core.MigratableRepository)
		if !ok {
			return fmt.Errorf("target backend %q не поддерживает ImportPosts", migrateTo)
		}

		existingCount, err := mig.Count(ctx)
		if err != nil {
			return fmt.Errorf("ошибка подсчёта постов в target: %w", err)
		}
		if existingCount > 0 && !migrateOverwrite {
			return fmt.Errorf("target содержит %d постов. Используйте --overwrite для перезаписи", existingCount)
		}

		fmt.Println("\n🔄 Начало миграции...")
		if err := mig.ImportPosts(ctx, allPosts); err != nil {
			return fmt.Errorf("ошибка импорта постов: %w", err)
		}

		newCount, err := mig.Count(ctx)
		if err != nil {
			return fmt.Errorf("ошибка подсчёта постов после миграции: %w", err)
		}

		fmt.Printf("\n✅ Миграция завершена успешно!\n📊 Мигрировано постов: %d\n", newCount)
		return nil
	},
}

func init() {
	migrateCmd.Flags().StringVar(&migrateFrom, "from", "", "source backend (fs|sqlite|postgres)")
	migrateCmd.Flags().StringVar(&migrateTo, "to", "", "target backend (fs|sqlite|postgres)")
	migrateCmd.Flags().BoolVarP(&migrateDryRun, "dry-run", "n", false, "режим предпросмотра без миграции")
	migrateCmd.Flags().BoolVarP(&migrateOverwrite, "overwrite", "f", false, "перезаписать target если уже содержит посты")
	// Скрытый legacy-флаг — для отдельной обработки в RunE с явным exit(2).
	migrateCmd.Flags().String("db", "", "")
	_ = migrateCmd.Flags().MarkHidden("db")
}

func isValidStorageType(t string) bool {
	return t == "fs" || t == "sqlite" || t == "postgres"
}

// loadConfigForMigrate загружает конфигурацию для команды migrate.
func loadConfigForMigrate(path string) (*config.Config, error) {
	cfg, err := config.Load(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg = config.NewDefaultConfig()
			if path != "" {
				if err := cfg.Save(path); err != nil {
					return nil, fmt.Errorf("ошибка сохранения конфигурации: %w", err)
				}
			}
			return cfg, nil
		}
		return nil, err
	}
	return cfg, nil
}
