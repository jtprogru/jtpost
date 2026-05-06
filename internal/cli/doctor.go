package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/adapters/telegram"
	"github.com/jtprogru/jtpost/internal/core"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:           "doctor",
	Short:         "Диагностика конфигурации и доступности зависимостей",
	SilenceUsage:  true,
	SilenceErrors: true,
	Long: `Проверяет, готов ли jtpost к работе:
  • найден ли конфиг и валиден ли YAML;
  • доступна ли директория постов на чтение/запись;
  • можно ли использовать SQLite-базу;
  • отвечает ли Telegram Bot API на токен из конфига;
  • задана ли переменная VISUAL/EDITOR.

Возвращает код 0, если все критичные проверки пройдены.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		return runDoctor(cmd.Context(), cmd.OutOrStdout(), configPath)
	},
}

type checkLevel int

const (
	levelOK checkLevel = iota
	levelWarn
	levelFail
)

type checkResult struct {
	level   checkLevel
	name    string
	message string
}

func runDoctor(ctx context.Context, out io.Writer, configPath string) error {
	results := []checkResult{
		{level: levelOK, name: "Версия", message: fmt.Sprintf("%s (commit %s, %s)", version, shortCommit(commit), date)},
	}

	cfg, cfgRes := checkConfig(configPath)
	results = append(results, cfgRes)

	if cfg != nil {
		results = append(results, checkPostsDir(cfg.PostsDir))
		results = append(results, checkSQLite(cfg.SQLite.DSN))
		results = append(results, checkTelegram(ctx, cfg.Telegram))
		results = append(results, checkEditor())
	}

	fmt.Fprintln(out, "🩺 jtpost doctor")
	fmt.Fprintln(out)
	hasFail := false
	for _, r := range results {
		fmt.Fprintf(out, "  %s %s: %s\n", icon(r.level), r.name, r.message)
		if r.level == levelFail {
			hasFail = true
		}
	}
	fmt.Fprintln(out)
	if hasFail {
		fmt.Fprintln(out, "❌ Часть критичных проверок провалена")
		return errors.New("doctor: проверки не пройдены")
	}
	fmt.Fprintln(out, "✅ Все проверки пройдены")
	return nil
}

func icon(l checkLevel) string {
	switch l {
	case levelOK:
		return "✓"
	case levelWarn:
		return "⚠"
	case levelFail:
		return "✗"
	}
	return "?"
}

func shortCommit(c string) string {
	if len(c) > 7 {
		return c[:7]
	}
	return c
}

func checkConfig(path string) (*config.Config, checkResult) {
	cfg, err := config.Load(path)
	if err != nil {
		if errors.Is(err, core.ErrConfigNotFound) {
			return nil, checkResult{
				level:   levelFail,
				name:    "Конфигурация",
				message: fmt.Sprintf("файл %s не найден — запусти `jtpost init`", path),
			}
		}
		return nil, checkResult{
			level:   levelFail,
			name:    "Конфигурация",
			message: fmt.Sprintf("ошибка загрузки %s: %v", path, err),
		}
	}
	return cfg, checkResult{
		level:   levelOK,
		name:    "Конфигурация",
		message: fmt.Sprintf("%s (валидный YAML)", path),
	}
}

func checkPostsDir(dir string) checkResult {
	if dir == "" {
		return checkResult{level: levelFail, name: "Директория постов", message: "не задана в конфиге (posts_dir)"}
	}
	abs, _ := filepath.Abs(dir)
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return checkResult{level: levelFail, name: "Директория постов", message: fmt.Sprintf("%s не существует", abs)}
		}
		return checkResult{level: levelFail, name: "Директория постов", message: err.Error()}
	}
	if !info.IsDir() {
		return checkResult{level: levelFail, name: "Директория постов", message: fmt.Sprintf("%s не директория", abs)}
	}
	tmp, err := os.CreateTemp(dir, ".jtpost-doctor-*")
	if err != nil {
		return checkResult{level: levelFail, name: "Директория постов", message: fmt.Sprintf("%s не доступна на запись: %v", abs, err)}
	}
	_ = tmp.Close()
	_ = os.Remove(tmp.Name())
	return checkResult{level: levelOK, name: "Директория постов", message: fmt.Sprintf("%s (rw)", abs)}
}

func checkSQLite(dsn string) checkResult {
	if dsn == "" {
		return checkResult{level: levelWarn, name: "SQLite", message: "не настроен (sqlite.dsn пуст)"}
	}
	abs, _ := filepath.Abs(dsn)
	dir := filepath.Dir(dsn)
	if _, err := os.Stat(dir); err != nil {
		return checkResult{level: levelWarn, name: "SQLite", message: fmt.Sprintf("каталог %s недоступен: %v", dir, err)}
	}
	if _, err := os.Stat(dsn); err == nil {
		return checkResult{level: levelOK, name: "SQLite", message: fmt.Sprintf("%s (файл найден)", abs)}
	}
	return checkResult{level: levelOK, name: "SQLite", message: fmt.Sprintf("%s (будет создан при первой операции)", abs)}
}

func checkTelegram(ctx context.Context, tg config.TelegramConfig) checkResult {
	if tg.BotToken == "" && tg.ChatID == "" {
		return checkResult{level: levelWarn, name: "Telegram", message: "не настроен — публикация в Telegram недоступна"}
	}
	if tg.BotToken == "" {
		return checkResult{level: levelFail, name: "Telegram", message: "bot_token не указан"}
	}
	if tg.ChatID == "" {
		return checkResult{level: levelFail, name: "Telegram", message: "chat_id не указан"}
	}
	pub := telegram.NewPublisher(telegram.Config{BotToken: tg.BotToken, ChannelID: tg.ChatID})
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	info, err := pub.GetMe(cctx)
	if err != nil {
		return checkResult{level: levelFail, name: "Telegram", message: fmt.Sprintf("getMe: %v", err)}
	}
	return checkResult{level: levelOK, name: "Telegram", message: fmt.Sprintf("бот @%s (id=%d)", info.Username, info.ID)}
}

func checkEditor() checkResult {
	for _, env := range []string{"VISUAL", "EDITOR"} {
		if v := os.Getenv(env); v != "" {
			return checkResult{level: levelOK, name: "Редактор", message: fmt.Sprintf("$%s=%s", env, v)}
		}
	}
	return checkResult{level: levelWarn, name: "Редактор", message: "VISUAL/EDITOR не заданы, по умолчанию vim"}
}
