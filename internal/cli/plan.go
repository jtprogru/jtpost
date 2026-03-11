package cli

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/jtprogru/jtpost/internal/adapters/fsrepo"
	"github.com/jtprogru/jtpost/internal/core"
	"github.com/spf13/cobra"
)

var planDays int

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "План публикаций",
	Long:  `Показывает план публикаций на ближайшие дни.`,
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

		// Получаем все посты
		posts, err := service.ListPosts(cmd.Context(), core.PostFilter{})
		if err != nil {
			return fmt.Errorf("ошибка получения списка постов: %w", err)
		}

		// Фильтруем по дедлайнам и запланированным датам
		now := time.Now()
		deadline := now.AddDate(0, 0, planDays)

		var plannedPosts []*plannedPost
		for _, post := range posts {
			if post.Status == core.StatusPublished {
				continue
			}

			var date *time.Time
			var dateType string

			if post.ScheduledAt != nil {
				date = post.ScheduledAt
				dateType = "schedule"
			} else if post.Deadline != nil {
				date = post.Deadline
				dateType = "deadline"
			}

			if date != nil && !date.After(deadline) {
				plannedPosts = append(plannedPosts, &plannedPost{
					Post:     post,
					Date:     *date,
					DateType: dateType,
				})
			}
		}

		// Сортируем по дате
		sortByDate(plannedPosts)

		// Выводим план
		printPlan(plannedPosts)

		return nil
	},
}

func init() {
	planCmd.Flags().IntVarP(&planDays, "days", "d", 30, "период планирования в днях")
}

type plannedPost struct {
	Post     *core.Post
	Date     time.Time
	DateType string
}

func sortByDate(posts []*plannedPost) {
	for i := 0; i < len(posts)-1; i++ {
		for j := i + 1; j < len(posts); j++ {
			if posts[j].Date.Before(posts[i].Date) {
				posts[i], posts[j] = posts[j], posts[i]
			}
		}
	}
}

func printPlan(posts []*plannedPost) {
	if len(posts) == 0 {
		fmt.Println("📅 Нет запланированных постов")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "DATE\tTYPE\tSTATUS\tTITLE\tPLATFORMS")
	fmt.Fprintln(w, "----\t----\t------\t-----\t---------")

	for _, p := range posts {
		dateStr := p.Date.Format("2006-01-02")
		typeStr := "⏰"
		if p.DateType == "deadline" {
			typeStr = "📋"
		}

		platforms := ""
		for i, plat := range p.Post.Platforms {
			if i > 0 {
				platforms += ", "
			}
			platforms += string(plat)
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			dateStr,
			typeStr,
			p.Post.Status,
			truncateString(p.Post.Title, 25),
			platforms,
		)
	}

	w.Flush()
	fmt.Printf("\n📊 Всего постов в плане: %d\n", len(posts))
}
