package cli

import (
	"encoding/json"
	"fmt"

	"github.com/jtprogru/jtpost/internal/adapters/fsrepo"
	"github.com/jtprogru/jtpost/internal/core"
	"github.com/spf13/cobra"
)

var showJSON bool

var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Показать детали поста",
	Long:  `Выводит подробную информацию о посте по его идентификатору.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := core.PostID(args[0])

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

		// Получаем пост
		post, err := service.GetByID(cmd.Context(), id)
		if err != nil {
			return fmt.Errorf("ошибка получения поста: %w", err)
		}

		if showJSON {
			data, err := json.MarshalIndent(post, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
		} else {
			printPostDetails(post)
		}

		return nil
	},
}

func init() {
	showCmd.Flags().BoolVarP(&showJSON, "json", "j", false, "вывод в формате JSON")
}

func printPostDetails(post *core.Post) {
	fmt.Printf("📝 Пост: %s\n\n", post.Title)
	fmt.Printf("  ID:       %s\n", post.ID)
	fmt.Printf("  Slug:     %s\n", post.Slug)
	fmt.Printf("  Статус:   %s\n", post.Status)
	fmt.Printf("  Платформы: %v\n", post.Platforms)
	fmt.Printf("  Теги:     %v\n", post.Tags)

	if post.Deadline != nil {
		fmt.Printf("  Deadline: %s\n", post.Deadline.Format("2006-01-02"))
	}
	if post.ScheduledAt != nil {
		fmt.Printf("  Заплан:   %s\n", post.ScheduledAt.Format("2006-01-02 15:04"))
	}
	if post.PublishedAt != nil {
		fmt.Printf("  Опублик:  %s\n", post.PublishedAt.Format("2006-01-02 15:04"))
	}

	if post.External.TelegramURL != "" {
		fmt.Printf("  TG URL:   %s\n", post.External.TelegramURL)
	}

	fmt.Printf("\n📄 Контент (%d символов):\n", len(post.Content))
	fmt.Println("---")
	if len(post.Content) > 500 {
		fmt.Println(post.Content[:500])
		fmt.Println("... (показано первые 500 символов)")
	} else {
		fmt.Println(post.Content)
	}
}
