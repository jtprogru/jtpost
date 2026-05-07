package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/jtprogru/jtpost/internal/adapters/apiclient"
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
		// F5c: --remote mode.
		didRun, err := runRemote(cmd, func(ctx context.Context, cli *apiclient.ClientWithResponses) error {
			return runNextRemote(ctx, cli, cmd.OutOrStdout())
		})
		if err != nil || didRun {
			return err
		}

		// Загружаем конфигурацию
		configPath, _ := cmd.Flags().GetString("config")
		cfg, err := loadConfigOrCreateDefault(configPath)
		if err != nil {
			return err
		}

		// Создаём репозиторий
		repo, closer, err := openRepo(cfg)
		if err != nil {
			return fmt.Errorf("ошибка создания репозитория: %w", err)
		}
		defer closer.Close()

		// Создаём сервис
		service := core.NewPostService(repo, core.SystemClock{})

		// Получаем рекомендацию
		ctx := scopeContext(cmd.Context(), cfg.Auth.TenantDefault, cfg.Auth.AuthorDefault)
		post, err := service.GetNextPost(ctx, cfg.Auth.TenantDefault)
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
