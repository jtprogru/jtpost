package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/jtprogru/jtpost/internal/adapters/fsrepo"
	"github.com/jtprogru/jtpost/internal/core"
	"github.com/spf13/cobra"
)

var (
	listStatuses  []string
	listPlatforms []string
	listTags      []string
	listSearch    string
	listFormat    string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Список постов",
	Long:  `Выводит список постов с возможностью фильтрации по статусу, платформам и тегам.`,
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

		// Преобразуем фильтры
		filter := core.PostFilter{
			Search: listSearch,
		}

		for _, s := range listStatuses {
			filter.Statuses = append(filter.Statuses, core.PostStatus(s))
		}

		for _, p := range listPlatforms {
			filter.Platforms = append(filter.Platforms, core.Platform(p))
		}

		filter.Tags = listTags

		// Получаем список постов
		posts, err := service.ListPosts(cmd.Context(), filter)
		if err != nil {
			return fmt.Errorf("ошибка получения списка постов: %w", err)
		}

		// Выводим результат
		switch listFormat {
		case "table":
			printTable(posts)
		case "json":
			// TODO: реализовать JSON вывод
			fmt.Printf("JSON формат будет реализован позже\n")
		default:
			printTable(posts)
		}

		return nil
	},
}

func init() {
	listCmd.Flags().StringSliceVarP(&listStatuses, "status", "s", []string{}, "фильтр по статусам")
	listCmd.Flags().StringSliceVarP(&listPlatforms, "platform", "P", []string{}, "фильтр по платформам")
	listCmd.Flags().StringSliceVarP(&listTags, "tag", "t", []string{}, "фильтр по тегам")
	listCmd.Flags().StringVarP(&listSearch, "search", "q", "", "поиск по заголовку/slug")
	listCmd.Flags().StringVarP(&listFormat, "format", "f", "table", "формат вывода (table, json)")
}

func printTable(posts []*core.Post) {
	if len(posts) == 0 {
		fmt.Println("Посты не найдены")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "STATUS\tTITLE\tSLUG\tPLATFORMS\tTAGS")
	fmt.Fprintln(w, "------\t-----\t----\t---------\t----")

	for _, post := range posts {
		platforms := ""
		for i, p := range post.Platforms {
			if i > 0 {
				platforms += ", "
			}
			platforms += string(p)
		}

		tags := ""
		for i, t := range post.Tags {
			if i > 0 {
				tags += ", "
			}
			tags += t
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			post.Status,
			truncateString(post.Title, 30),
			post.Slug,
			platforms,
			tags,
		)
	}

	w.Flush()
	fmt.Printf("\n📊 Всего постов: %d\n", len(posts))
}

func truncateString(s string, maxLen int) string {
	if maxLen <= 3 {
		return "..."
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
