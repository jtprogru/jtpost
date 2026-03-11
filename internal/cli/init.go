package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Инициализация проекта jtpost",
	Long:  `Создаёт файл конфигурации .jtpost.yaml с настройками по умолчанию.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")

		// Проверяем, существует ли уже файл
		if _, err := os.Stat(configPath); err == nil {
			return fmt.Errorf("файл %s уже существует, удалите его для повторной инициализации", configPath)
		}

		// Создаём конфигурацию по умолчанию
		cfg := config.NewDefaultConfig()

		// Получаем абсолютный путь для сохранения
		absPath, err := filepath.Abs(configPath)
		if err != nil {
			return fmt.Errorf("ошибка получения абсолютного пути: %w", err)
		}

		// Сохраняем конфигурацию
		if err := cfg.Save(absPath); err != nil {
			return fmt.Errorf("ошибка сохранения конфигурации: %w", err)
		}

		// Создаём директорию для постов если не существует
		if err := os.MkdirAll(cfg.PostsDir, 0o755); err != nil {
			return fmt.Errorf("ошибка создания директории постов: %w", err)
		}

		// Создаём директорсию для шаблонов
		if err := os.MkdirAll(cfg.TemplatesDir, 0o755); err != nil {
			return fmt.Errorf("ошибка создания директории шаблонов: %w", err)
		}

		fmt.Printf("✅ Проект jtpost инициализирован!\n\n")
		fmt.Printf("📁 Конфигурация: %s\n", absPath)
		fmt.Printf("📁 Директория постов: %s\n", cfg.PostsDir)
		fmt.Printf("📁 Директория шаблонов: %s\n", cfg.TemplatesDir)
		fmt.Printf("\n📝 Следующий шаг: создайте первый пост командой\n")
		fmt.Printf("   jtpost new \"Заголовок поста\"\n")

		return nil
	},
}
