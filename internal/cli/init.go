package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/spf13/cobra"
)

var initForce bool

// uuidGenerator абстрагирует генерацию UUID для тестируемости.
type uuidGenerator interface {
	New() uuid.UUID
}

// realUUIDGen генерирует UUID v7 (с фолбэком на v4 при ошибке).
type realUUIDGen struct{}

// New реализует uuidGenerator.
func (realUUIDGen) New() uuid.UUID {
	u, err := uuid.NewV7()
	if err != nil {
		return uuid.New()
	}
	return u
}

// initUUIDGen — подменяемый генератор UUID для тестов.
var initUUIDGen uuidGenerator = realUUIDGen{}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Инициализация проекта jtpost",
	Long:  `Создаёт файл конфигурации .jtpost.yaml с настройками по умолчанию и генерирует UUIDv7 для tenant_default/author_default.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		absPath, err := filepath.Abs(configPath)
		if err != nil {
			return fmt.Errorf("ошибка получения абсолютного пути: %w", err)
		}

		// Если файл уже существует и не указан --force, спрашиваем пользователя.
		if _, statErr := os.Stat(absPath); statErr == nil && !initForce {
			fmt.Fprintf(cmd.OutOrStdout(), "Config already exists at %s. Overwrite? [y/N]: ", absPath)
			reader := bufio.NewReader(cmd.InOrStdin())
			line, _ := reader.ReadString('\n')
			line = strings.TrimSpace(line)
			if line == "" || (line[0] != 'y' && line[0] != 'Y') {
				fmt.Fprintln(cmd.ErrOrStderr(), "Aborted")
				return nil
			}
		}

		// Генерируем tenant + author UUIDs c уникальным префиксом.
		tenantID := initUUIDGen.New()
		var authorID uuid.UUID
		tenantShort := shortID(tenantID)
		const maxAttempts = 10
		attempt := maxAttempts
		for i := range maxAttempts {
			authorID = initUUIDGen.New()
			if shortID(authorID) != tenantShort {
				attempt = i
				break
			}
		}
		if attempt == maxAttempts {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: tenant/author short IDs collide after %d attempts\n", maxAttempts)
		}

		cfg := config.NewDefaultConfig()
		cfg.Auth.TenantDefault = tenantID
		cfg.Auth.AuthorDefault = authorID

		if err := cfg.Save(absPath); err != nil {
			return fmt.Errorf("ошибка сохранения конфигурации: %w", err)
		}

		// Создаём директории <posts_dir>/<tenant_short>/ и <templates_dir>.
		tenantPostsDir := filepath.Join(cfg.PostsDir, tenantShort)
		if err := os.MkdirAll(tenantPostsDir, 0o755); err != nil {
			return fmt.Errorf("ошибка создания директории постов: %w", err)
		}
		if err := os.MkdirAll(cfg.TemplatesDir, 0o755); err != nil {
			return fmt.Errorf("ошибка создания директории шаблонов: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "✅ Проект jtpost инициализирован!\n\n")
		fmt.Fprintf(cmd.OutOrStdout(), "📁 Конфигурация: %s\n", absPath)
		fmt.Fprintf(cmd.OutOrStdout(), "📁 Директория постов: %s\n", tenantPostsDir)
		fmt.Fprintf(cmd.OutOrStdout(), "📁 Директория шаблонов: %s\n", cfg.TemplatesDir)
		fmt.Fprintf(cmd.OutOrStdout(), "🔑 tenant_default: %s\n", tenantID)
		fmt.Fprintf(cmd.OutOrStdout(), "🔑 author_default: %s\n", authorID)

		return nil
	},
}

func init() {
	initCmd.Flags().BoolVar(&initForce, "force", false, "перезаписать существующий конфиг без подтверждения")
}

// shortID возвращает первые 8 hex-символов UUID без дефисов.
func shortID(id uuid.UUID) string {
	s := strings.ReplaceAll(id.String(), "-", "")
	if len(s) < 8 {
		return s
	}
	return s[:8]
}
