package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/core"
	"github.com/spf13/cobra"
)

var (
	tokenCreateUserID    string
	tokenCreateName      string
	tokenCreateExpiresIn time.Duration
	tokenListUserID      string
)

var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Управление Personal Access Tokens",
	Long:  `Команды для создания, просмотра и отзыва PAT (требует storage.type=sqlite|postgres).`,
}

var tokenCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Создать новый PAT",
	RunE: func(cmd *cobra.Command, args []string) error {
		bundle, cfg, err := openBundleForAuth(cmd)
		if err != nil {
			return err
		}
		defer bundle.Closer.Close()

		if tokenCreateUserID == "" || tokenCreateName == "" {
			return fmt.Errorf("--user-id и --name обязательны")
		}
		userID, err := uuid.Parse(tokenCreateUserID)
		if err != nil {
			return fmt.Errorf("invalid --user-id: %w", err)
		}

		// Проверим, что user существует
		if _, err := bundle.Users.GetByID(context.Background(), userID); err != nil {
			return fmt.Errorf("user not found: %s", userID)
		}

		var expPtr *time.Duration
		if tokenCreateExpiresIn > 0 {
			expPtr = &tokenCreateExpiresIn
		}

		svc := core.NewAuthService(bundle.Users, bundle.Tokens, cfg.Auth.BCryptCost, core.SystemClock{})
		issued, err := svc.IssueToken(context.Background(), userID, tokenCreateName, expPtr)
		if err != nil {
			return err
		}
		fmt.Printf("✅ Token created\n")
		fmt.Printf("   ID:     %s\n", issued.Token.ID)
		fmt.Printf("   Name:   %s\n", issued.Token.Name)
		fmt.Printf("   Prefix: %s\n", issued.Token.Prefix)
		fmt.Printf("\n🔑 Token (save it now, will not be shown again):\n\n")
		fmt.Printf("   %s\n\n", issued.Raw)
		fmt.Printf("⚠️  Use it as: Authorization: Bearer %s\n", issued.Raw)
		return nil
	},
}

var tokenListCmd = &cobra.Command{
	Use:   "list",
	Short: "Список PAT пользователя",
	RunE: func(cmd *cobra.Command, args []string) error {
		bundle, _, err := openBundleForAuth(cmd)
		if err != nil {
			return err
		}
		defer bundle.Closer.Close()

		if tokenListUserID == "" {
			return fmt.Errorf("--user-id обязателен")
		}
		userID, err := uuid.Parse(tokenListUserID)
		if err != nil {
			return fmt.Errorf("invalid --user-id: %w", err)
		}
		tokens, err := bundle.Tokens.ListByUser(context.Background(), userID)
		if err != nil {
			return err
		}
		if len(tokens) == 0 {
			fmt.Println("PAT не найдены")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tPREFIX\tCREATED\tEXPIRES\tLAST USED")
		fmt.Fprintln(w, "--\t----\t------\t-------\t-------\t---------")
		for _, t := range tokens {
			expires := "-"
			if t.ExpiresAt != nil {
				expires = t.ExpiresAt.Format("2006-01-02")
			}
			lastUsed := "-"
			if t.LastUsedAt != nil {
				lastUsed = t.LastUsedAt.Format("2006-01-02 15:04")
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				t.ID, t.Name, t.Prefix,
				t.CreatedAt.Format("2006-01-02"), expires, lastUsed)
		}
		w.Flush()
		fmt.Printf("\n📊 Всего: %d\n", len(tokens))
		return nil
	},
}

var tokenRevokeCmd = &cobra.Command{
	Use:   "revoke <token-id>",
	Short: "Отозвать PAT",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		bundle, _, err := openBundleForAuth(cmd)
		if err != nil {
			return err
		}
		defer bundle.Closer.Close()

		tokenID, err := uuid.Parse(args[0])
		if err != nil {
			return fmt.Errorf("invalid token-id: %w", err)
		}
		if err := bundle.Tokens.Delete(context.Background(), tokenID); err != nil {
			return err
		}
		fmt.Printf("✅ Token %s revoked\n", tokenID)
		return nil
	},
}

func init() {
	tokenCreateCmd.Flags().StringVar(&tokenCreateUserID, "user-id", "", "UUID пользователя")
	tokenCreateCmd.Flags().StringVar(&tokenCreateName, "name", "", "имя токена (для идентификации)")
	tokenCreateCmd.Flags().DurationVar(&tokenCreateExpiresIn, "expires-in", 0, "время до истечения (например 90d=2160h); 0 = без истечения")

	tokenListCmd.Flags().StringVar(&tokenListUserID, "user-id", "", "UUID пользователя")

	tokenCmd.AddCommand(tokenCreateCmd)
	tokenCmd.AddCommand(tokenListCmd)
	tokenCmd.AddCommand(tokenRevokeCmd)
}
