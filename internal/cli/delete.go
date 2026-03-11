package cli

import (
	"fmt"

	"github.com/jtprogru/jtpost/internal/adapters/fsrepo"
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
		id := core.PostID(args[0])

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

		// Получаем пост для отображения информации
		post, err := service.GetByID(cmd.Context(), id)
		if err != nil {
			return fmt.Errorf("ошибка получения поста: %w", err)
		}

		// Запрашиваем подтверждение, если не указан --force
		if !deleteForce {
			fmt.Printf("📝 Пост: %s\n", post.Title)
			fmt.Printf("   Статус: %s\n", post.Status)
			fmt.Printf("   Платформы: %v\n\n", post.Platforms)
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
		if err := service.DeletePost(cmd.Context(), id); err != nil {
			return fmt.Errorf("ошибка удаления поста: %w", err)
		}

		fmt.Printf("✅ Пост удалён: %s\n", post.Title)

		return nil
	},
}

func init() {
	deleteCmd.Flags().BoolVarP(&deleteForce, "force", "f", false, "удалить без подтверждения")
}
