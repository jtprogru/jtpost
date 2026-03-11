package cli

import (
	"fmt"

	"github.com/jtprogru/jtpost/internal/adapters/fsrepo"
	"github.com/jtprogru/jtpost/internal/core"
	"github.com/spf13/cobra"
)

var statusSet string

var statusCmd = &cobra.Command{
	Use:   "status <id>",
	Short: "Смена статуса поста",
	Long:  `Изменяет статус поста. Доступные статусы: idea, draft, ready, scheduled, published.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := core.PostID(args[0])

		if statusSet == "" {
			return fmt.Errorf("укажите новый статус через флаг --set")
		}

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

		// Преобразуем статус
		newStatus := core.PostStatus(statusSet)
		if !isValidStatus(newStatus) {
			return fmt.Errorf("%w: недопустимый статус '%s' (допустимы: idea, draft, ready, scheduled, published)", core.ErrInvalidStatus, statusSet)
		}

		// Обновляем статус
		post, err := service.UpdateStatus(cmd.Context(), id, newStatus)
		if err != nil {
			return fmt.Errorf("ошибка обновления статуса: %w", err)
		}

		fmt.Printf("✅ Статус поста изменён: %s → %s\n", post.Status, newStatus)

		return nil
	},
}

func init() {
	statusCmd.Flags().StringVarP(&statusSet, "set", "s", "", "новый статус поста")
	_ = statusCmd.MarkFlagRequired("set")
}

func isValidStatus(status core.PostStatus) bool {
	valid := []core.PostStatus{
		core.StatusIdea,
		core.StatusDraft,
		core.StatusReady,
		core.StatusScheduled,
		core.StatusPublished,
	}
	for _, s := range valid {
		if s == status {
			return true
		}
	}
	return false
}
