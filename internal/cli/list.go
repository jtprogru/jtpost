package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/jtprogru/jtpost/internal/adapters/apiclient"
	"github.com/jtprogru/jtpost/internal/core"
	"github.com/spf13/cobra"
)

var (
	listStatuses []string
	listTags     []string
	listSearch   string
	listFormat   string
	listNoID     bool
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Список постов",
	Long:  `Выводит список постов с возможностью фильтрации по статусу и тегам.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Remote-mode (F5b/c): если задан --remote → используем apiclient.
		didRun, err := runRemote(cmd, func(ctx context.Context, cli *apiclient.ClientWithResponses) error {
			return runListRemote(ctx, cli, cmd.OutOrStdout())
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

		// Преобразуем фильтры
		filter := core.PostFilter{
			TenantID: cfg.Auth.TenantDefault,
			Search:   listSearch,
		}

		for _, s := range listStatuses {
			filter.Statuses = append(filter.Statuses, core.PostStatus(s))
		}

		filter.Tags = listTags

		// Получаем список постов
		ctx := scopeContext(cmd.Context(), cfg.Auth.TenantDefault, cfg.Auth.AuthorDefault)
		posts, err := service.ListPosts(ctx, filter)
		if err != nil {
			return fmt.Errorf("ошибка получения списка постов: %w", err)
		}

		showID := !listNoID

		// Выводим результат
		switch listFormat {
		case "json":
			return printPostsJSON(cmd.OutOrStdout(), posts)
		case "table":
			printTable(posts, showID)
		default:
			printTable(posts, showID)
		}

		return nil
	},
}

func init() {
	listCmd.Flags().StringSliceVarP(&listStatuses, "status", "s", []string{}, "фильтр по статусам")
	listCmd.Flags().StringSliceVarP(&listTags, "tag", "t", []string{}, "фильтр по тегам")
	listCmd.Flags().StringVarP(&listSearch, "search", "q", "", "поиск по заголовку/slug")
	listCmd.Flags().StringVarP(&listFormat, "format", "f", "table", "формат вывода (table, json)")
	listCmd.Flags().BoolVarP(&listNoID, "no-id", "", false, "скрыть колонку ID")
}

// printPostsJSON выводит posts как JSON-массив. Для пустого ввода возвращает "[]\n".
func printPostsJSON(out interface{ Write(p []byte) (int, error) }, posts []*core.Post) error {
	if len(posts) == 0 {
		posts = []*core.Post{}
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(posts)
}

func printTable(posts []*core.Post, showID bool) {
	if len(posts) == 0 {
		fmt.Println("Посты не найдены")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	if showID {
		fmt.Fprintln(w, "ID\tSTATUS\tTITLE\tSLUG\tTAGS")
		fmt.Fprintln(w, "--\t------\t-----\t----\t----")
	} else {
		fmt.Fprintln(w, "STATUS\tTITLE\tSLUG\tTAGS")
		fmt.Fprintln(w, "------\t-----\t----\t----")
	}

	for _, post := range posts {
		tags := ""
		for i, t := range post.Tags {
			if i > 0 {
				tags += ", "
			}
			tags += t
		}

		if showID {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				post.ID,
				post.Status,
				truncateString(post.Title, 30),
				post.Slug,
				tags,
			)
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				post.Status,
				truncateString(post.Title, 30),
				post.Slug,
				tags,
			)
		}
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
