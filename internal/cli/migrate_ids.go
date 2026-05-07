package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/adapters/fsrepo"
	"github.com/jtprogru/jtpost/internal/core"
	"github.com/spf13/cobra"
)

var (
	migrateIDsForce    bool
	migrateIDsPostsDir string
)

// oldIDPattern регулярное выражение для старого формата ID: timestamp-slug.
// Пример: 1710345678-my-post-slug.
var oldIDPattern = regexp.MustCompile(`^(\d+)-[a-z0-9-]+$`)

var migrateIDsCmd = &cobra.Command{
	Use:   "migrate-ids",
	Short: "Миграция старых ID постов на UUID v7",
	Long:  `Конвертирует посты со старым форматом ID (timestamp-slug) в UUID v7.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		// Загружаем конфигурацию
		configPath, _ := cmd.Flags().GetString("config")
		cfg, err := loadConfigForMigrateIDs(configPath)
		if err != nil {
			return err
		}

		// Определяем директорию с постами
		postsDir := migrateIDsPostsDir
		if postsDir == "" {
			postsDir = cfg.PostsDir
		}

		fmt.Printf("🔄 Миграция ID постов на UUID v7\n")
		fmt.Printf("═══════════════════════════════════════\n")
		fmt.Printf("Директория: %s\n", postsDir)
		fmt.Printf("Принудительно: %v\n", migrateIDsForce)
		fmt.Println()

		// Создаём файловый репозиторий
		fsRepo, err := fsrepo.NewFileSystemRepository(postsDir)
		if err != nil {
			return fmt.Errorf("ошибка создания файлового репозитория: %w", err)
		}

		// Получаем все посты
		ctx = scopeContext(ctx, cfg.Auth.TenantDefault, cfg.Auth.AuthorDefault)
		allPosts, err := fsRepo.List(ctx, core.PostFilter{TenantID: cfg.Auth.TenantDefault})
		if err != nil {
			return fmt.Errorf("ошибка получения списка постов: %w", err)
		}

		fmt.Printf("📊 Найдено постов: %d\n", len(allPosts))

		// Находим посты со старыми ID
		var postsToMigrate []*core.Post
		for _, post := range allPosts {
			if hasOldIDFormat(post.ID.String()) {
				postsToMigrate = append(postsToMigrate, post)
			}
		}

		if len(postsToMigrate) == 0 {
			fmt.Println("✅ Все посты уже используют UUID v7")
			return nil
		}

		fmt.Printf("📋 Посты для миграции: %d\n\n", len(postsToMigrate))

		// Показываем список постов для миграции
		for _, post := range postsToMigrate {
			newID := convertOldIDToUUIDv7(post.ID.String())
			fmt.Printf("  • %s\n", post.Title)
			fmt.Printf("    Старый ID: %s\n", post.ID.String())
			fmt.Printf("    Новый ID:  %s\n", newID.String())
			fmt.Println()
		}

		if !migrateIDsForce {
			fmt.Println("⚠️  Это режим предпросмотра. Используйте --force для реальной миграции")
			return nil
		}

		// Мигрируем посты
		fmt.Println("🔄 Начало миграции...")
		fmt.Println()

		migrated := 0
		errors := 0

		for _, post := range postsToMigrate {
			newID := convertOldIDToUUIDv7(post.ID.String())

			// Обновляем ID поста
			post.ID = newID

			// Сериализуем и записываем файл
			data, err := fsrepo.SerializePostWithFrontmatter(post)
			if err != nil {
				fmt.Printf("❌ Ошибка сериализации поста %s: %v\n", post.Title, err)
				errors++
				continue
			}

			filePath := filepath.Join(postsDir, post.TenantShortID(), post.Slug+".md")
			if err := os.WriteFile(filePath, data, 0o600); err != nil {
				fmt.Printf("❌ Ошибка записи файла %s: %v\n", post.Slug, err)
				errors++
				continue
			}

			fmt.Printf("✅ %s → %s\n", post.ID.String(), newID.String())
			migrated++
		}

		fmt.Println()
		fmt.Printf("═══════════════════════════════════════\n")
		fmt.Printf("✅ Миграция завершена!\n")
		fmt.Printf("📊 Мигрировано: %d\n", migrated)
		if errors > 0 {
			fmt.Printf("❌ Ошибки: %d\n", errors)
		}

		return nil
	},
}

func init() {
	migrateIDsCmd.Flags().BoolVarP(&migrateIDsForce, "force", "f", false, "выполнить миграцию без предпросмотра")
	migrateIDsCmd.Flags().StringVarP(&migrateIDsPostsDir, "dir", "d", "", "директория с постами (переопределяет конфиг)")
}

// loadConfigForMigrateIDs загружает конфигурацию для команды migrate-ids.
func loadConfigForMigrateIDs(path string) (*config.Config, error) {
	cfg, err := config.Load(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("конфигурация не найдена: %w. Сначала выполните 'jtpost init'", err)
		}
		return nil, err
	}
	return cfg, nil
}

// hasOldIDFormat проверяет, имеет ли ID старый формат (timestamp-slug).
func hasOldIDFormat(idStr string) bool {
	return oldIDPattern.MatchString(idStr)
}

// convertOldIDToUUIDv7 конвертирует старый ID (timestamp-slug) в UUID v7.
// Извлекает timestamp из старого ID и генерирует UUID v7 на его основе.
func convertOldIDToUUIDv7(oldID string) core.PostID {
	matches := oldIDPattern.FindStringSubmatch(oldID)
	if len(matches) < 2 {
		// Если не удалось распарсить, генерируем новый UUID
		return core.GeneratePostID("", time.Now())
	}

	timestamp, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		// Если не удалось распарсить timestamp, генерируем новый UUID
		return core.GeneratePostID("", time.Now())
	}

	// Конвертируем Unix timestamp в time.Time
	t := time.Unix(timestamp, 0)

	// Генерируем UUID v7 на основе timestamp
	// uuid.NewV7() использует текущее время, но мы можем создать UUID v7 вручную
	u := generateUUIDv7FromTime(t)

	return core.PostID(u)
}

// generateUUIDv7FromTime генерирует UUID v7 из указанного времени.
// UUID v7 формат: https://www.rfc-editor.org/rfc/rfc9562.html#section-5.7
// 48 бит timestamp (мс) + 4 бита версии (0111) + 62 бита случайности.
func generateUUIDv7FromTime(t time.Time) uuid.UUID {
	// Получаем timestamp в миллисекундах
	ts := t.UnixMilli()

	// Создаём UUID v7 используя uuid.NewV7() с кастомным временем
	// uuid.NewV7() генерирует UUID v7 на основе текущего времени
	// Но мы можем создать UUID вручную, установив правильные биты

	var u uuid.UUID

	// Заполняем первые 6 байт timestamp (48 бит)
	for i := range 6 {
		u[i] = byte((ts >> (40 - i*8)) & 0xFF)
	}

	// Байт 6: младшие 12 бит timestamp (старшие 4 бита) + версия
	u[6] = byte((ts >> 16) & 0x0F) // Младшие 4 бита timestamp
	u[6] |= 0x70                   // Версия 7 (0111)

	// Байт 7: старшие 8 бит рандома (используем младшие 8 бита timestamp)
	u[7] = byte(ts & 0xFF)

	// Заполняем остальные байты случайными данными
	// Для простоты используем uuid.New() для генерации случайной части
	randomUUID := uuid.New()
	for i := 8; i < 16; i++ {
		u[i] = randomUUID[i]
	}

	// Устанавливаем вариант RFC 4122 (биты 6-7 байта 8 = 10)
	u[8] = (u[8] & 0x3F) | 0x80

	return u
}
