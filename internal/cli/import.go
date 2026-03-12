package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jtprogru/jtpost/internal/adapters/fsrepo"
	"github.com/jtprogru/jtpost/internal/core"
	"github.com/spf13/cobra"
)

var (
	importDryRun     bool
	importInteractive bool
	importOutput     string
	importUpdate     bool
)

var importCmd = &cobra.Command{
	Use:   "import [source-dir]",
	Short: "Импортировать посты из существующих Markdown файлов",
	Long: `Импортирует посты из указанной директории (или content/posts/ по умолчанию).
Сканирует Markdown файлы, парсит frontmatter, нормализует к стандарту jtpost.

Флаги:
  --dry-run        Показать, что будет импортировано, без записи
  --interactive    Запрашивать подтверждение для каждого файла
  --output         Директория для импортированных файлов (по умолчанию postsDir из конфига)
  --update         Обновлять существующие посты вместо пропуска
`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Определяем источник
		sourceDir := "content/posts"
		if len(args) > 0 {
			sourceDir = args[0]
		}

		// Загружаем конфигурацию
		configPath, _ := cmd.Flags().GetString("config")
		cfg, err := loadConfigOrCreateDefault(configPath)
		if err != nil {
			return err
		}

		// Определяем выходную директорию
		outputDir := cfg.PostsDir
		if importOutput != "" {
			outputDir = importOutput
		}

		// Создаём репозиторий для выходной директории
		outputRepo, err := fsrepo.NewFileSystemRepository(outputDir)
		if err != nil {
			return fmt.Errorf("ошибка создания репозитория: %w", err)
		}

		// Создаём сервис
		service := core.NewPostService(outputRepo, core.SystemClock{})

		fmt.Printf("🔍 Сканирование директории: %s\n", sourceDir)
		fmt.Printf("📁 Выходная директория: %s\n", outputDir)
		fmt.Printf("🔧 Режим: %s\n", getModeString())
		fmt.Println()

		// Сканируем файлы
		markdownFiles, err := scanMarkdownFiles(sourceDir)
		if err != nil {
			return fmt.Errorf("ошибка сканирования файлов: %w", err)
		}

		if len(markdownFiles) == 0 {
			fmt.Println("⚠️  Markdown файлы не найдены")
			return nil
		}

		fmt.Printf("📄 Найдено файлов: %d\n\n", len(markdownFiles))

		// Обрабатываем файлы
		stats := &fsrepo.ImportStats{}
		for _, filePath := range markdownFiles {
			if err := processFile(cmd.Context(), filePath, service, stats); err != nil {
				fmt.Printf("❌ Ошибка обработки %s: %v\n", filePath, err)
				stats.Errors++
			}
		}

		// Вывод статистики
		printStats(stats)

		return nil
	},
}

func init() {
	importCmd.Flags().BoolVarP(&importDryRun, "dry-run", "n", false, "режим предпросмотра без записи")
	importCmd.Flags().BoolVarP(&importInteractive, "interactive", "i", false, "интерактивный режим с подтверждениями")
	importCmd.Flags().StringVarP(&importOutput, "output", "o", "", "выходная директория (по умолчанию postsDir из конфига)")
	importCmd.Flags().BoolVarP(&importUpdate, "update", "u", false, "обновлять существующие посты")
}

// getModeString возвращает строку текущего режима.
func getModeString() string {
	if importDryRun {
		return "DRY-RUN"
	}
	if importInteractive {
		return "INTERACTIVE"
	}
	return "AUTO"
}

// scanMarkdownFiles сканирует директорию на наличие Markdown файлов.
func scanMarkdownFiles(dir string) ([]string, error) {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

// processFile обрабатывает один файл импорта.
func processFile(ctx context.Context, filePath string, service *core.PostService, stats *fsrepo.ImportStats) error {
	stats.Total++

	// Читаем файл
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("ошибка чтения файла: %w", err)
	}

	// Парсим frontmatter
	result, err := fsrepo.ParseFrontmatter(string(content))
	if err != nil {
		return fmt.Errorf("ошибка парсинга frontmatter: %w", err)
	}

	// Извлекаем slug из имени файла
	slug := strings.TrimSuffix(filepath.Base(filePath), ".md")
	// Удаляем префикс даты если есть (YYYY-MM-DD-)
	slug = removeDatePrefix(slug)

	// Нормализуем frontmatter
	post, err := fsrepo.NormalizeFrontmatter(result, slug)
	if err != nil {
		return fmt.Errorf("ошибка нормализации frontmatter: %w", err)
	}

	// Проверяем, существует ли уже пост
	existingPost, err := service.GetBySlug(ctx, post.Slug)
	if err == nil && existingPost != nil {
		if !importUpdate {
			fmt.Printf("⏭️  Пропущено: %s (уже существует)\n", post.Slug)
			stats.Skipped++
			return nil
		}
		// Обновляем существующий пост
		post.ID = existingPost.ID
	}

	// Вывод информации о файле
	fmt.Printf("📝 Файл: %s\n", filepath.Base(filePath))
	fmt.Printf("   Slug: %s\n", post.Slug)
	fmt.Printf("   Заголовок: %s\n", post.Title)
	fmt.Printf("   Статус: %s\n", post.Status)
	fmt.Printf("   Платформы: %v\n", post.Platforms)
	fmt.Printf("   Frontmatter: %s\n", frontmatterTypeString(result.Type))

	// Генерируем ID если нет
	if post.ID == "" {
		post.ID = core.GeneratePostID(post.Slug, time.Now())
	}

	// Режим dry-run — только показываем
	if importDryRun {
		fmt.Printf("   📋 [DRY-RUN] Будет создан/обновлён\n")
		fmt.Println()
		stats.Imported++
		return nil
	}

	// Интерактивный режим — запрашиваем подтверждение
	if importInteractive {
		if !confirmImport() {
			fmt.Printf("   ⏭️  Пропущено по запросу пользователя\n")
			fmt.Println()
			stats.Skipped++
			return nil
		}
	}

	// Создаём/обновляем пост
	var action string
	if existingPost != nil && importUpdate {
		if err := service.UpdatePost(ctx, post); err != nil {
			return fmt.Errorf("ошибка обновления поста: %w", err)
		}
		action = "updated"
		stats.Updated++
	} else {
		if err := service.CreatePostWithContent(ctx, post); err != nil {
			return fmt.Errorf("ошибка создания поста: %w", err)
		}
		action = "created"
		stats.Imported++
	}

	fmt.Printf("   ✅ %s: %s\n", strings.ToUpper(string(action[0]))+action[1:], post.Title)
	fmt.Println()

	return nil
}

// removeDatePrefix удаляет префикс даты из slug.
func removeDatePrefix(slug string) string {
	// Пробуем удалить префикс YYYY-MM-DD-
	if len(slug) > 11 && slug[4] == '-' && slug[7] == '-' {
		parts := strings.SplitN(slug, "-", 4)
		if len(parts) == 4 {
			// Проверяем, что первые 3 части — это дата
			if _, err := time.Parse("2006-01-02", parts[0]+"-"+parts[1]+"-"+parts[2]); err == nil {
				return parts[3]
			}
		}
	}
	return slug
}

// frontmatterTypeString возвращает строковое представление типа frontmatter.
func frontmatterTypeString(t fsrepo.FrontmatterType) string {
	switch t {
	case fsrepo.FrontmatterYAML:
		return "YAML"
	case fsrepo.FrontmatterTOML:
		return "TOML"
	default:
		return "отсутствует"
	}
}

// confirmImport запрашивает подтверждение импорта.
func confirmImport() bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("   Импортировать? [Y/n]: ")

	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	input = strings.TrimSpace(strings.ToLower(input))
	return input == "" || input == "y" || input == "yes"
}

// printStats выводит статистику импорта.
func printStats(stats *fsrepo.ImportStats) {
	fmt.Println("=====================================")
	fmt.Println("📊 Статистика импорта:")
	fmt.Printf("   Всего файлов: %d\n", stats.Total)
	fmt.Printf("   Импортировано: %d\n", stats.Imported)
	fmt.Printf("   Обновлено: %d\n", stats.Updated)
	fmt.Printf("   Пропущено: %d\n", stats.Skipped)
	fmt.Printf("   Ошибки: %d\n", stats.Errors)
	fmt.Println("=====================================")
}
