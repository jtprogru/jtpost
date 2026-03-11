package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/jtprogru/jtpost/internal/adapters/fsrepo"
	"github.com/jtprogru/jtpost/internal/core"
	"github.com/spf13/cobra"
)

var (
	nextFormat string
)

var nextCmd = &cobra.Command{
	Use:   "next",
	Short: "Рекомендация следующего поста",
	Long:  `Рекомендует следующий пост для работы на основе дедлайнов, scheduled_at и статуса.`,
	RunE: func(cmd *cobra.Command, args []string) error {
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

		// Получаем рекомендацию
		post, err := service.GetNextPost(cmd.Context())
		if err != nil {
			return fmt.Errorf("ошибка получения рекомендации: %w", err)
		}

		if post == nil {
			fmt.Println("Нет постов для рекомендации — все посты опубликованы или запланированы")
			return nil
		}

		// Выводим результат
		switch nextFormat {
		case "json":
			return printNextJSON(post)
		case "plain":
			fmt.Println(post.Slug)
			return nil
		case "full":
			printNextFull(post)
			return nil
		default:
			printNextFull(post)
			return nil
		}
	},
}

func init() {
	nextCmd.Flags().StringVarP(&nextFormat, "format", "f", "full", "формат вывода (full, plain, json)")
}

func printNextJSON(post *core.Post) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(post)
}

func printNextFull(post *core.Post) {
	fmt.Println("📌 Рекомендуемый пост для работы:")
	fmt.Println("==================================")
	fmt.Printf("\n📝 Заголовок: %s\n", post.Title)
	fmt.Printf("🏷️ Slug: %s\n", post.Slug)
	fmt.Printf("📊 Статус: %s\n", post.Status)

	if post.Deadline != nil {
		fmt.Printf("⏰ Дедлайн: %s\n", post.Deadline.Format("2006-01-02 15:04"))
	} else {
		fmt.Println("⏰ Дедлайн: не установлен")
	}

	if post.ScheduledAt != nil {
		fmt.Printf("📅 Запланировано: %s\n", post.ScheduledAt.Format("2006-01-02 15:04"))
	}

	// Платформы
	var platformsBuilder strings.Builder
	for i, p := range post.Platforms {
		if i > 0 {
			platformsBuilder.WriteString(", ")
		}
		platformsBuilder.WriteString(string(p))
	}
	fmt.Printf("🌐 Платформы: %s\n", platformsBuilder.String())

	// Теги
	var tagsBuilder strings.Builder
	for i, t := range post.Tags {
		if i > 0 {
			tagsBuilder.WriteString(", ")
		}
		tagsBuilder.WriteString(t)
	}
	tagsStr := tagsBuilder.String()
	if tagsStr != "" {
		fmt.Printf("🏷️ Теги: %s\n", tagsStr)
	}

	fmt.Println()
}
