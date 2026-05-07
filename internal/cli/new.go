package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/adapters/fsrepo"
	"github.com/jtprogru/jtpost/internal/core"
	"github.com/spf13/cobra"
)

var (
	newTags   []string
	newSlug   string
	newEditor string
	newTenant string
	newAuthor string
)

var newCmd = &cobra.Command{
	Use:   "new [title]",
	Short: "Создание нового поста",
	Long:  `Создаёт новый пост с указанным заголовком и открывает его в редакторе.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		title := args[0]

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

		// Определяем tenant/author: приоритет --tenant/--author > cfg.Auth.*
		tenantID := cfg.Auth.TenantDefault
		if newTenant != "" {
			parsed, err := uuid.Parse(newTenant)
			if err != nil {
				return fmt.Errorf("invalid UUID for --tenant: %w", err)
			}
			tenantID = parsed
		}
		authorID := cfg.Auth.AuthorDefault
		if newAuthor != "" {
			parsed, err := uuid.Parse(newAuthor)
			if err != nil {
				return fmt.Errorf("invalid UUID for --author: %w", err)
			}
			authorID = parsed
		}

		ctx := scopeContext(cmd.Context(), tenantID, authorID)

		// Создаём пост
		post, err := service.CreatePost(ctx, core.CreatePostInput{
			TenantID: tenantID,
			AuthorID: authorID,
			Title:    title,
			Tags:     newTags,
			Slug:     newSlug,
		})
		if err != nil {
			return fmt.Errorf("ошибка создания поста: %w", err)
		}

		// Строим путь к файлу (с учётом tenant subdir)
		filePath := filepath.Join(cfg.PostsDir, post.TenantShortID(), fmt.Sprintf("%s.md", post.Slug))

		fmt.Printf("✅ Пост создан: %s\n", post.Title)
		fmt.Printf("📁 Файл: %s\n", filePath)
		fmt.Printf("🏷️  Статус: %s\n", post.Status)

		// Открываем в редакторе
		if err := openInEditor(filePath, newEditor); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Не удалось открыть редактор: %v\n", err)
		}

		return nil
	},
}

func init() {
	newCmd.Flags().StringSliceVarP(&newTags, "tag", "t", []string{}, "теги поста")
	newCmd.Flags().StringVarP(&newSlug, "slug", "s", "", "slug поста (по умолчанию генерируется из заголовка)")
	newCmd.Flags().StringVarP(&newEditor, "editor", "e", "", "редактор для открытия файла (по умолчанию $VISUAL или $EDITOR)")
	newCmd.Flags().StringVar(&newTenant, "tenant", "", "tenant UUID (по умолчанию из конфига)")
	newCmd.Flags().StringVar(&newAuthor, "author", "", "author UUID (по умолчанию из конфига)")
}

// tenantShortHex возвращает первые 8 hex-символов TenantID без дефисов.
func tenantShortHex(id uuid.UUID) string {
	s := strings.ReplaceAll(id.String(), "-", "")
	if len(s) < 8 {
		return s
	}
	return s[:8]
}

// scopeContext возвращает контекст с установленными tenant/author scope.
func scopeContext(ctx context.Context, tenantID, authorID uuid.UUID) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if tenantID != uuid.Nil {
		ctx = core.WithTenant(ctx, tenantID)
	}
	if authorID != uuid.Nil {
		ctx = core.WithAuthor(ctx, authorID)
	}
	return ctx
}

// loadConfigOrCreateDefault загружает конфигурацию или создаёт дефолтную.
func loadConfigOrCreateDefault(path string) (*config.Config, error) {
	cfg, err := config.Load(path)
	if err != nil {
		if errors.Is(err, core.ErrConfigNotFound) {
			// Создаём конфигурацию по умолчанию
			cfg = config.NewDefaultConfig()
			fmt.Printf("⚠️  Конфигурация не найдена, используем значения по умолчанию\n")
			fmt.Printf("📁 Директория постов: %s\n", cfg.PostsDir)
		} else {
			return nil, err
		}
	}
	return cfg, nil
}

// openInEditor открывает файл в редакторе.
func openInEditor(filePath, editor string) error {
	if editor == "" {
		editor = os.Getenv("VISUAL")
		if editor == "" {
			editor = os.Getenv("EDITOR")
		}
		if editor == "" {
			// Платформенно-зависимый редактор по умолчанию
			editor = "vim"
		}
	}

	// Разделяем команду и аргументы
	parts := strings.Fields(editor)
	cmd := exec.Command(parts[0], append(parts[1:], filePath)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// createPostTemplate создаёт шаблон контента для нового поста.
func createPostTemplate(title string, createdAt time.Time) string {
	var sb strings.Builder

	sb.WriteString("# ")
	sb.WriteString(title)
	sb.WriteString("\n\n")
	sb.WriteString("<!-- Начало поста -->\n\n")
	sb.WriteString("Ваш контент здесь...\n")

	return sb.String()
}
