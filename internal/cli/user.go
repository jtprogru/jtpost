package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/adapters/storage"
	"github.com/jtprogru/jtpost/internal/core"
	"github.com/spf13/cobra"
)

var (
	userCreateEmail      string
	userCreatePassword   string
	userCreateRole       string
	userCreateFirstOwner bool
)

var userCmd = &cobra.Command{
	Use:   "user",
	Short: "Управление пользователями",
	Long:  `Команды управления учётными записями (требует storage.type=sqlite|postgres).`,
}

var userCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Создать пользователя",
	RunE: func(cmd *cobra.Command, args []string) error {
		bundle, cfg, err := openBundleForAuth(cmd)
		if err != nil {
			return err
		}
		defer bundle.Closer.Close()

		if userCreateEmail == "" || userCreatePassword == "" {
			return fmt.Errorf("--email и --password обязательны")
		}

		ctx := context.Background()
		count, err := bundle.Users.Count(ctx, cfg.Auth.TenantDefault)
		if err != nil {
			return fmt.Errorf("count users: %w", err)
		}

		if userCreateFirstOwner {
			if count > 0 {
				return fmt.Errorf("first owner already exists (users count = %d)", count)
			}
			userCreateRole = string(core.RoleOwner)
		} else {
			if count == 0 {
				return fmt.Errorf("no users yet — use --first-owner to bootstrap the first user")
			}
		}

		role := core.Role(userCreateRole)
		if role == "" {
			role = core.RoleAuthor
		}
		svc := core.NewAuthService(bundle.Users, bundle.Tokens, bundle.Sessions, cfg.Auth.BCryptCost, core.SystemClock{})
		user, err := svc.CreateUser(ctx, core.CreateUserInput{
			TenantID: cfg.Auth.TenantDefault,
			Email:    userCreateEmail,
			Password: userCreatePassword,
			Role:     role,
		})
		if err != nil {
			return err
		}
		fmt.Printf("✅ User created\n")
		fmt.Printf("   ID:    %s\n", user.ID)
		fmt.Printf("   Email: %s\n", user.Email)
		fmt.Printf("   Role:  %s\n", user.Role)
		return nil
	},
}

var userListCmd = &cobra.Command{
	Use:   "list",
	Short: "Список пользователей текущего tenant",
	RunE: func(cmd *cobra.Command, args []string) error {
		bundle, cfg, err := openBundleForAuth(cmd)
		if err != nil {
			return err
		}
		defer bundle.Closer.Close()

		users, err := bundle.Users.List(context.Background(), cfg.Auth.TenantDefault)
		if err != nil {
			return err
		}
		if len(users) == 0 {
			fmt.Println("Пользователи не найдены")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tEMAIL\tROLE\tCREATED")
		fmt.Fprintln(w, "--\t-----\t----\t-------")
		for _, u := range users {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", u.ID, u.Email, u.Role, u.CreatedAt.Format("2006-01-02"))
		}
		w.Flush()
		fmt.Printf("\n📊 Всего: %d\n", len(users))
		return nil
	},
}

var userDeleteCmd = &cobra.Command{
	Use:   "delete <user-id>",
	Short: "Удалить пользователя (caskade удалит токены)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		bundle, _, err := openBundleForAuth(cmd)
		if err != nil {
			return err
		}
		defer bundle.Closer.Close()

		userID, err := uuid.Parse(args[0])
		if err != nil {
			return fmt.Errorf("invalid user-id: %w", err)
		}

		ctx := context.Background()
		user, err := bundle.Users.GetByID(ctx, userID)
		if err != nil {
			if errors.Is(err, core.ErrNotFound) {
				return fmt.Errorf("user %s не найден", userID)
			}
			return err
		}
		if user.Role == core.RoleOwner {
			cnt, err := bundle.Users.CountOwners(ctx, user.TenantID)
			if err != nil {
				return err
			}
			if cnt <= 1 {
				return fmt.Errorf("cannot delete last owner")
			}
		}
		if err := bundle.Users.Delete(ctx, userID); err != nil {
			return err
		}
		fmt.Printf("✅ User %s deleted\n", userID)
		return nil
	},
}

func openBundleForAuth(cmd *cobra.Command) (*storage.Bundle, *config.Config, error) {
	configPath, _ := cmd.Flags().GetString("config")
	cfg, err := loadConfigOrCreateDefault(configPath)
	if err != nil {
		return nil, nil, err
	}
	bundle, err := storage.OpenBundle(cfg)
	if err != nil {
		return nil, nil, err
	}
	if bundle.Users == nil || bundle.Tokens == nil {
		_ = bundle.Closer.Close()
		return nil, nil, fmt.Errorf("user management requires storage.type=sqlite or postgres")
	}
	return bundle, cfg, nil
}

func init() {
	userCreateCmd.Flags().StringVar(&userCreateEmail, "email", "", "email пользователя")
	userCreateCmd.Flags().StringVar(&userCreatePassword, "password", "", "пароль (≥ 8 символов)")
	userCreateCmd.Flags().StringVar(&userCreateRole, "role", "author", "роль: owner|editor|author|viewer")
	userCreateCmd.Flags().BoolVar(&userCreateFirstOwner, "first-owner", false, "создать первого owner (только при пустой users-таблице)")

	userCmd.AddCommand(userCreateCmd)
	userCmd.AddCommand(userListCmd)
	userCmd.AddCommand(userDeleteCmd)
}
