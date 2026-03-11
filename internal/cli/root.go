package cli

import (
	"fmt"
	"os"

	"github.com/jtprogru/jtpost/internal/core"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// SetVersion устанавливает версию CLI.
func SetVersion(v, c, d string) {
	version = v
	commit = c
	date = d
}

// Execute запускает корневую команду CLI.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "jtpost",
	Short: "CLI-редактор постов для Telegram",
	Long:  `jtpost — утилита для управления жизненным циклом постов: от идеи до публикации в Telegram.`,
	Version: fmt.Sprintf("%s (commit: %s, date: %s)", version, commit, date),
}

func init() {
	cobra.OnInitialize(initConfig)

	// Глобальные флаги
	rootCmd.PersistentFlags().StringP("config", "c", ".jtpost.yaml", "путь к конфигурационному файлу")
	rootCmd.PersistentFlags().StringP("posts-dir", "D", "", "директория с постами (переопределяет конфиг)")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "подробный вывод")

	// Подкоманды
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(newCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(showCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(editCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(publishCmd)
	rootCmd.AddCommand(planCmd)
	rootCmd.AddCommand(statsCmd)
	rootCmd.AddCommand(nextCmd)
	rootCmd.AddCommand(serveCmd)
}

func initConfig() {
	// Инициализация конфигурации будет здесь
	// Пока оставляем пустым для базовой работы
}

// getService создаёт и возвращает PostService для использования в командах.
// Это временная реализация до полной интеграции с адаптерами.
func getService() (*core.PostService, error) {
	// TODO: загрузить конфиг, создать репозиторий, вернуть сервис
	return nil, fmt.Errorf("сервис ещё не инициализирован — используйте команду init")
}
