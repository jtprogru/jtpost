package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jtprogru/jtpost/internal/adapters/fsrepo"
	"github.com/jtprogru/jtpost/internal/core"
	"github.com/spf13/cobra"
)

var editEditor string

var editCmd = &cobra.Command{
	Use:   "edit <id>",
	Short: "Редактировать пост",
	Long:  `Открывает файл поста в редакторе для редактирования.`,
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

		// Получаем пост для проверки существования
		_, err = repo.GetByID(cmd.Context(), id)
		if err != nil {
			return fmt.Errorf("пост не найден: %w", err)
		}

		// Находим файл
		filePath, err := findPostFile(cfg.PostsDir, id)
		if err != nil {
			return err
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
}

func findPostFile(postsDir string, id core.PostID) (string, error) {
	entries, err := os.ReadDir(postsDir)
	if err != nil {
		return "", err
	}

	idStr := string(id)
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
