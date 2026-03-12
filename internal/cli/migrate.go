package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/adapters/fsrepo"
	"github.com/jtprogru/jtpost/internal/adapters/sqlite"
	"github.com/jtprogru/jtpost/internal/core"
	"github.com/spf13/cobra"
)

var (
	migrateDBPath     string
	migrateDryRun     bool
	migrateOverwrite  bool
	migrateSourceDir  string
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Миграция постов из файлового хранилища в SQLite",
	Long:  `Мигрирует все посты из файлового хранилища в базу данных SQLite.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		// Загружаем конфигурацию
		configPath, _ := cmd.Flags().GetString("config")
		cfg, err := loadConfigForMigrate(configPath)
		if err != nil {
			return err
		}

		// Определяем путь к БД
		dbPath := migrateDBPath
		if dbPath == "" {
			dbPath = cfg.SQLite.DSN
		}
		if dbPath == "" {
			dbPath = ".jtpost.db"
		}

		// Определяем источник
		sourceDir := migrateSourceDir
		if sourceDir == "" {
			sourceDir = cfg.PostsDir
		}

		fmt.Printf("📦 Миграция постов\n")
		fmt.Printf("═══════════════════════════════════════\n")
		fmt.Printf("Источник: %s\n", sourceDir)
		fmt.Printf("Назначение: %s\n", dbPath)
		fmt.Printf("Сухой запуск: %v\n", migrateDryRun)
		fmt.Println()

		// Создаём файловый репозиторий (источник)
		fsRepo, err := fsrepo.NewFileSystemRepository(sourceDir)
		if err != nil {
			return fmt.Errorf("ошибка создания файлового репозитория: %w", err)
		}

		// Получаем все посты из источника
		allPosts, err := fsRepo.List(ctx, core.PostFilter{})
		if err != nil {
			return fmt.Errorf("ошибка получения списка постов: %w", err)
		}

		fmt.Printf("📊 Найдено постов: %d\n", len(allPosts))

		if len(allPosts) == 0 {
			fmt.Println("✅ Нечего мигрировать — посты не найдены")
			return nil
		}

		// Сухой запуск — показываем что будет мигрировано
		if migrateDryRun {
			fmt.Println("\n📋 Посты для миграции:")
			for _, post := range allPosts {
				fmt.Printf("  • %s (%s) — %s\n", post.Title, post.Slug, post.Status)
			}
			fmt.Println("\n⚠️  Это режим предпросмотра. Уберите --dry-run для реальной миграции")
			return nil
		}

		// Создаём SQLite репозиторий (назначение)
		absDBPath, err := filepath.Abs(dbPath)
		if err != nil {
			return fmt.Errorf("ошибка получения абсолютного пути к БД: %w", err)
		}

		sqliteRepo, err := sqlite.NewSQLitePostRepository(sqlite.Config{DSN: absDBPath})
		if err != nil {
			return fmt.Errorf("ошибка создания SQLite репозитория: %w", err)
		}
		defer sqliteRepo.Close()

		// Проверяем, есть ли уже посты в БД
		existingCount, err := sqliteRepo.Count(ctx)
		if err != nil {
			return fmt.Errorf("ошибка подсчёта постов в БД: %w", err)
		}

		if existingCount > 0 && !migrateOverwrite {
			return fmt.Errorf("база данных уже содержит %d постов. Используйте --overwrite для перезаписи", existingCount)
		}

		// Мигрируем посты
		fmt.Println("\n🔄 Начало миграции...")

		if err := sqliteRepo.ImportPosts(ctx, allPosts); err != nil {
			return fmt.Errorf("ошибка импорта постов: %w", err)
		}

		// Проверяем результат
		newCount, err := sqliteRepo.Count(ctx)
		if err != nil {
			return fmt.Errorf("ошибка подсчёта постов после миграции: %w", err)
		}

		fmt.Println()
		fmt.Printf("✅ Миграция завершена успешно!\n")
		fmt.Printf("📊 Мигрировано постов: %d\n", newCount)
		fmt.Printf("💾 База данных: %s\n", absDBPath)

		return nil
	},
}

func init() {
	migrateCmd.Flags().StringVarP(&migrateDBPath, "db", "d", "", "путь к файлу SQLite БД (.jtpost.db)")
	migrateCmd.Flags().BoolVarP(&migrateDryRun, "dry-run", "n", false, "режим предпросмотра без миграции")
	migrateCmd.Flags().BoolVarP(&migrateOverwrite, "overwrite", "f", false, "перезаписать существующую БД")
	migrateCmd.Flags().StringVarP(&migrateSourceDir, "from", "s", "", "директория с постами для импорта")
}

// loadConfigForMigrate загружает конфигурацию для команды migrate.
func loadConfigForMigrate(path string) (*config.Config, error) {
	cfg, err := config.Load(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Создаём дефолтную конфигурацию
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
