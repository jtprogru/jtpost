package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"text/tabwriter"

	"github.com/jtprogru/jtpost/internal/adapters/fsrepo"
	"github.com/jtprogru/jtpost/internal/core"
	"github.com/spf13/cobra"
)

var (
	statsFormat string
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Статистика по постам",
	Long:  `Выводит статистику по постам: количество по статусам, платформам и тегам.`,
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

		// Получаем статистику
		stats, err := service.GetStats(cmd.Context())
		if err != nil {
			return fmt.Errorf("ошибка получения статистики: %w", err)
		}

		// Выводим результат
		switch statsFormat {
		case "json":
			return printStatsJSON(stats)
		case "table":
			printStatsTable(stats)
			return nil
		default:
			printStatsTable(stats)
			return nil
		}
	},
}

func init() {
	statsCmd.Flags().StringVarP(&statsFormat, "format", "f", "table", "формат вывода (table, json)")
}

func printStatsTable(stats *core.PostStats) {
	fmt.Println("📊 Статистика постов")
	fmt.Println("====================")
	fmt.Printf("\n📈 Всего постов: %d\n", stats.Total)

	// Статусы
	fmt.Println("\n📋 По статусам:")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  СТАТУС\tКОЛИЧЕСТВО")
	fmt.Fprintln(w, "  ------\t--------")

	// Выводим в порядке жизненного цикла
	for _, status := range core.StatusOrder {
		if count, ok := stats.ByStatus[status]; ok {
			fmt.Fprintf(w, "  %s\t%d\n", status, count)
		}
	}

	// Статусы, которых нет в StatusOrder (на случай расширения)
	for status, count := range stats.ByStatus {
		if !slices.Contains(core.StatusOrder, status) {
			fmt.Fprintf(w, "  %s\t%d\n", status, count)
		}
	}

	w.Flush()

	// Платформы
	fmt.Println("\n🌐 По платформам:")
	w = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  ПЛАТФОРМА\tКОЛИЧЕСТВО")
	fmt.Fprintln(w, "  ---------\t--------")

	platforms := make([]core.Platform, 0, len(stats.ByPlatform))
	for platform := range stats.ByPlatform {
		platforms = append(platforms, platform)
	}
	slices.SortFunc(platforms, func(a, b core.Platform) int {
		if string(a) < string(b) {
			return -1
		}
		if string(a) > string(b) {
			return 1
		}
		return 0
	})

	for _, platform := range platforms {
		fmt.Fprintf(w, "  %s\t%d\n", platform, stats.ByPlatform[platform])
	}
	w.Flush()

	// Теги
	fmt.Println("\n🏷️ По тегам:")
	w = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  ТЕГ\tКОЛИЧЕСТВО")
	fmt.Fprintln(w, "  ---\t--------")

	tags := make([]string, 0, len(stats.ByTag))
	for tag := range stats.ByTag {
		tags = append(tags, tag)
	}
	slices.Sort(tags)

	for _, tag := range tags {
		fmt.Fprintf(w, "  %s\t%d\n", tag, stats.ByTag[tag])
	}
	w.Flush()
	fmt.Println()
}

func printStatsJSON(stats *core.PostStats) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(stats)
}
