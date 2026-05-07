package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/adapters/storage"
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
	Use:     "jtpost",
	Short:   "CLI-редактор постов для Telegram",
	Long:    `jtpost — утилита для управления жизненным циклом постов: от идеи до публикации в Telegram.`,
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
	rootCmd.AddCommand(importCmd)
	rootCmd.AddCommand(migrateCmd)
	// migrateDBCmd регистрируется как subcommand migrateCmd в migrate_db.go init().
	rootCmd.AddCommand(migrateIDsCmd)
	rootCmd.AddCommand(doctorCmd)
}

func initConfig() {
	// Инициализация конфигурации будет здесь
	// Пока оставляем пустым для базовой работы
}

// openRepo конструирует core.PostRepository по cfg.Storage.Type. Caller
// обязан вызвать closer.Close() после использования.
func openRepo(cfg *config.Config) (core.PostRepository, io.Closer, error) {
	return storage.Open(cfg)
}

// openRepoAs аналогичен openRepo, но позволяет переопределить storage.type
// (используется командой `migrate --from --to`).
func openRepoAs(cfg *config.Config, storageType string) (core.PostRepository, io.Closer, error) {
	return storage.OpenAs(cfg, storageType)
}
