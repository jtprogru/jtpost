package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jtprogru/jtpost/internal/adapters/apiclient"
	"github.com/jtprogru/jtpost/internal/core"
	"github.com/spf13/cobra"
)

var (
	editEditor  string
	editTitle   string
	editContent string
	editTags    []string
	editStatus  string
)

var editCmd = &cobra.Command{
	Use:   "edit <id>",
	Short: "Редактировать пост",
	Long:  `Открывает файл поста в редакторе для редактирования.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// F5d2: --remote mode.
		hasTags := cmd.Flags().Changed("tag")
		didRun, err := runRemote(cmd, func(ctx context.Context, cli *apiclient.ClientWithResponses) error {
			if editEditor != "" {
				fmt.Fprintln(os.Stderr, "⚠️  --editor ignored in --remote mode")
			}
			return runEditRemote(ctx, cli, args[0], editTitle, editContent, editStatus, editTags, hasTags, os.Stdin, cmd.OutOrStdout())
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

		// Получаем пост для проверки существования
		ctx := scopeContext(cmd.Context(), cfg.Auth.TenantDefault, cfg.Auth.AuthorDefault)
		_, err = repo.GetByID(ctx, id)
		if err != nil {
			return fmt.Errorf("пост не найден: %w", err)
		}

		// Находим файл
		tenantShort := tenantShortHex(cfg.Auth.TenantDefault)
		filePath, err := findPostFile(filepath.Join(cfg.PostsDir, tenantShort), id)
		if err != nil {
			// Пробуем корень postsDir для обратной совместимости
			filePath, err = findPostFile(cfg.PostsDir, id)
			if err != nil {
				return err
			}
		}

		// Открываем в редакторе
		editor := editEditor
		if editor == "" {
			editor = os.Getenv("VISUAL")
			if editor == "" {
				editor = os.Getenv("EDITOR")
			}
			if editor == "" {
				editor = "vim"
			}
		}

		parts := strings.Fields(editor)
		cmdExec := exec.Command(parts[0], append(parts[1:], filePath)...)
		cmdExec.Stdin = os.Stdin
		cmdExec.Stdout = os.Stdout
		cmdExec.Stderr = os.Stderr

		if err := cmdExec.Run(); err != nil {
			return fmt.Errorf("ошибка запуска редактора: %w", err)
		}

		fmt.Printf("✅ Пост отредактирован: %s\n", filePath)

		return nil
	},
}

func init() {
	editCmd.Flags().StringVarP(&editEditor, "editor", "e", "", "редактор для открытия файла")
	editCmd.Flags().StringVar(&editTitle, "title", "", "новый заголовок (только для --remote)")
	editCmd.Flags().StringVar(&editContent, "content", "", "источник контента: '-' для stdin или путь к файлу (только для --remote)")
	editCmd.Flags().StringSliceVar(&editTags, "tag", nil, "новый набор тегов (replace; только для --remote)")
	editCmd.Flags().StringVar(&editStatus, "status", "", "новый статус: draft|ready|scheduled|published (только для --remote)")
}

func findPostFile(postsDir string, id core.PostID) (string, error) {
	entries, err := os.ReadDir(postsDir)
	if err != nil {
		return "", err
	}

	idStr := id.String()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		if strings.Contains(entry.Name(), idStr) {
			return filepath.Join(postsDir, entry.Name()), nil
		}
	}

	return "", core.ErrNotFound
}
