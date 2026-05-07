package cli

import (
	"context"
	"fmt"

	"github.com/jtprogru/jtpost/internal/adapters/apiclient"
	"github.com/jtprogru/jtpost/internal/core"
	"github.com/spf13/cobra"
)

var deleteForce bool

var deleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Удалить пост",
	Long:  `Удаляет пост по его идентификатору. Без флага --force запрашивает подтверждение.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// F5d: --remote mode.
		didRun, err := runRemote(cmd, func(ctx context.Context, cli *apiclient.ClientWithResponses) error {
			return runDeleteRemote(ctx, cli, args[0], cmd.OutOrStdout())
		})
		if err != nil || didRun {
			return err
		}

		id, err := core.ParsePostID(args[0])
		if err != nil {
			return fmt.Errorf("неверный формат ID: %w", err)
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

		// Получаем пост для отображения информации
		ctx := scopeContext(cmd.Context(), cfg.Auth.TenantDefault, cfg.Auth.AuthorDefault)
		post, err := service.GetByID(ctx, id)
		if err != nil {
			return fmt.Errorf("ошибка получения поста: %w", err)
		}

		// Запрашиваем подтверждение, если не указан --force
		if !deleteForce {
			fmt.Printf("📝 Пост: %s\n", post.Title)
			fmt.Printf("   Статус: %s\n\n", post.Status)
			fmt.Print("⚠️  Вы уверены, что хотите удалить этот пост? [y/N]: ")

			var response string
			if _, err := fmt.Scanln(&response); err != nil {
				return nil // Пользователь отменил (Ctrl+D или ошибка ввода)
			}

			if response != "y" && response != "Y" && response != "yes" {
				fmt.Println("❌ Удаление отменено")
				return nil
			}
		}

		// Удаляем пост
		if err := service.DeletePost(ctx, id); err != nil {
			return fmt.Errorf("ошибка удаления поста: %w", err)
		}

		fmt.Printf("✅ Пост удалён: %s\n", post.Title)

		return nil
	},
}

func init() {
	deleteCmd.Flags().BoolVarP(&deleteForce, "force", "f", false, "удалить без подтверждения")
}
