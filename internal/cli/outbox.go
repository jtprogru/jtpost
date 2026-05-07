package cli

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/adapters/storage"
	"github.com/jtprogru/jtpost/internal/core"
	"github.com/spf13/cobra"
)

var (
	outboxListStatus string
	outboxListLimit  int
)

var outboxCmd = &cobra.Command{
	Use:   "outbox",
	Short: "Управление очередью публикаций (outbox)",
}

var outboxEnqueueCmd = &cobra.Command{
	Use:   "enqueue <post-id>",
	Short: "Поставить пост в очередь на публикацию",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		postID, err := core.ParsePostID(args[0])
		if err != nil {
			return fmt.Errorf("неверный формат ID: %w", err)
		}
		configPath, _ := cmd.Flags().GetString("config")
		cfg, err := loadConfigOrCreateDefault(configPath)
		if err != nil {
			return err
		}
		bundle, err := storage.OpenBundle(cfg)
		if err != nil {
			return fmt.Errorf("open storage: %w", err)
		}
		defer bundle.Closer.Close()
		if bundle.Outbox == nil {
			return fmt.Errorf("outbox недоступен для backend %q", cfg.Storage.Type)
		}
		ctx := scopeContext(cmd.Context(), cfg.Auth.TenantDefault, cfg.Auth.AuthorDefault)
		post, err := bundle.Posts.GetByID(ctx, postID)
		if err != nil {
			return fmt.Errorf("пост не найден: %w", err)
		}
		entry, err := core.EnqueueForPublish(ctx, bundle.Outbox, post, time.Time{})
		if err != nil {
			return fmt.Errorf("ошибка enqueue: %w", err)
		}
		fmt.Printf("✅ Пост поставлен в очередь: entry=%s post=%s\n", entry.ID, postID)
		return nil
	},
}

var outboxListCmd = &cobra.Command{
	Use:   "list",
	Short: "Показать записи outbox",
	RunE: func(cmd *cobra.Command, _ []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		cfg, err := loadConfigOrCreateDefault(configPath)
		if err != nil {
			return err
		}
		bundle, err := storage.OpenBundle(cfg)
		if err != nil {
			return fmt.Errorf("open storage: %w", err)
		}
		defer bundle.Closer.Close()
		if bundle.Outbox == nil {
			return fmt.Errorf("outbox недоступен для backend %q", cfg.Storage.Type)
		}
		filter := core.OutboxFilter{Limit: outboxListLimit}
		if outboxListStatus != "" {
			filter.Status = core.OutboxStatus(outboxListStatus)
		}
		entries, err := bundle.Outbox.List(cmd.Context(), filter)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			fmt.Println("Очередь пуста")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tPOST\tSTATUS\tATTEMPTS\tNEXT_ATTEMPT\tLAST_ERROR")
		fmt.Fprintln(w, "--\t----\t------\t--------\t------------\t----------")
		for _, e := range entries {
			fmt.Fprintf(w, "%s\t%s\t%s\t%d/%d\t%s\t%s\n",
				e.ID, uuid.UUID(e.PostID), e.Status, e.Attempts, e.MaxAttempts,
				e.NextAttemptAt.Format(time.RFC3339), truncateString(e.LastError, 40))
		}
		w.Flush()
		fmt.Printf("\n📊 Всего: %d\n", len(entries))
		return nil
	},
}

func init() {
	outboxListCmd.Flags().StringVar(&outboxListStatus, "status", "", "фильтр по статусу (pending|in_flight|done|failed)")
	outboxListCmd.Flags().IntVar(&outboxListLimit, "limit", 50, "максимум записей")
	outboxCmd.AddCommand(outboxEnqueueCmd, outboxListCmd)
}
