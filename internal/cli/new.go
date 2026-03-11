package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/adapters/fsrepo"
	"github.com/jtprogru/jtpost/internal/core"
	"github.com/spf13/cobra"
)

var (
	newPlatforms []string
	newTags      []string
	newSlug      string
	newEditor    string
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

		// Преобразуем строки платформ в типы
		platforms, err := parsePlatforms(newPlatforms)
		if err != nil {
			return err
		}

		// Если платформы не указаны, используем значения по умолчанию
		if len(platforms) == 0 {
			for _, p := range cfg.Defaults.Platforms {
				platforms = append(platforms, core.Platform(p))
			}
			if len(platforms) == 0 {
				platforms = []core.Platform{core.PlatformTelegram}
			}
		}

		// Создаём пост
		post, err := service.CreatePost(cmd.Context(), core.CreatePostInput{
			Title:     title,
			Platforms: platforms,
			Tags:      newTags,
			Slug:      newSlug,
		})
		if err != nil {
			return fmt.Errorf("ошибка создания поста: %w", err)
		}

		// Строим путь к файлу
		filePath := filepath.Join(cfg.PostsDir, fmt.Sprintf("%s.md", post.Slug))

		fmt.Printf("✅ Пост создан: %s\n", post.Title)
		fmt.Printf("📁 Файл: %s\n", filePath)
		fmt.Printf("🏷️  Статус: %s\n", post.Status)
		fmt.Printf("📝 Платформы: %v\n", post.Platforms)

		// Открываем в редакторе
		if err := openInEditor(filePath, newEditor); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Не удалось открыть редактор: %v\n", err)
		}

		return nil
	},
}

func init() {
	newCmd.Flags().StringSliceVarP(&newPlatforms, "platform", "P", []string{}, "платформы публикации (telegram)")
	newCmd.Flags().StringSliceVarP(&newTags, "tag", "t", []string{}, "теги поста")
	newCmd.Flags().StringVarP(&newSlug, "slug", "s", "", "slug поста (по умолчанию генерируется из заголовка)")
	newCmd.Flags().StringVarP(&newEditor, "editor", "e", "", "редактор для открытия файла (по умолчанию $VISUAL или $EDITOR)")
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

// parsePlatforms преобразует строки платформ в типы.
func parsePlatforms(strs []string) ([]core.Platform, error) {
	var platforms []core.Platform
	for _, s := range strs {
		switch strings.ToLower(s) {
		case "telegram":
			platforms = append(platforms, core.PlatformTelegram)
		default:
			return nil, fmt.Errorf("%w: неизвестная платформа '%s' (допустима: telegram)", core.ErrInvalidPlatform, s)
		}
	}
	return platforms, nil
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
