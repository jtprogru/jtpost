package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
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
		results = append(results, checkStorage(ctx, cfg))
		if cfg.Storage.Type == "fs" && cfg.Storage.Git.Enabled {
			results = append(results, checkGitRepo(cfg)...)
		}
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

func checkStorage(ctx context.Context, cfg *config.Config) checkResult {
	st := cfg.Storage.Type
	if st == "" {
		st = "fs"
	}
	switch st {
	case "fs":
		// PostsDir уже проверен в checkPostsDir.
		return checkResult{level: levelOK, name: "Storage", message: "fs (используется PostsDir)"}
	case "sqlite":
		dsn := cfg.SQLiteDSN()
		if dsn == "" {
			return checkResult{level: levelFail, name: "Storage", message: "sqlite: storage.sqlite.dsn пуст"}
		}
		repo, closer, err := openRepo(cfg)
		if err != nil {
			return checkResult{level: levelFail, name: "Storage", message: fmt.Sprintf("sqlite open %s: %v", dsn, err)}
		}
		defer closer.Close()
		mig, ok := repo.(core.MigratableRepository)
		if !ok {
			return checkResult{level: levelOK, name: "Storage", message: fmt.Sprintf("sqlite (%s) — open ok", dsn)}
		}
		count, err := mig.Count(ctx)
		if err != nil {
			return checkResult{level: levelFail, name: "Storage", message: fmt.Sprintf("sqlite count: %v", err)}
		}
		return checkResult{level: levelOK, name: "Storage", message: fmt.Sprintf("sqlite (%s) — %d posts", dsn, count)}
	case "postgres":
		dsn := cfg.Storage.Postgres.DSN
		masked := maskDSN(dsn)
		if dsn == "" {
			return checkResult{level: levelFail, name: "Storage", message: "postgres: storage.postgres.dsn пуст"}
		}
		repo, closer, err := openRepo(cfg)
		if err != nil {
			return checkResult{level: levelFail, name: "Storage", message: fmt.Sprintf("postgres open %s: %v", masked, err)}
		}
		defer closer.Close()
		mig, ok := repo.(core.MigratableRepository)
		if !ok {
			return checkResult{level: levelOK, name: "Storage", message: fmt.Sprintf("postgres (%s) — open ok", masked)}
		}
		count, err := mig.Count(ctx)
		if err != nil {
			return checkResult{level: levelFail, name: "Storage", message: fmt.Sprintf("postgres count: %v", err)}
		}
		return checkResult{level: levelOK, name: "Storage", message: fmt.Sprintf("postgres (%s) — %d posts", masked, count)}
	default:
		return checkResult{level: levelFail, name: "Storage", message: fmt.Sprintf("unknown storage.type: %s", st)}
	}
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

// checkGitRepo проверяет состояние git-репозитория в posts_dir, когда
// storage.git.enabled=true. Возвращает 1-2 checkResult: статус репо
// и (если задан Remote) совпадение конфигурации remote.
func checkGitRepo(cfg *config.Config) []checkResult {
	results := []checkResult{}
	repo, err := git.PlainOpen(cfg.PostsDir)
	if err != nil {
		return []checkResult{{
			level:   levelFail,
			name:    "Git",
			message: fmt.Sprintf("не git-репозиторий (%s): %v", cfg.PostsDir, err),
		}}
	}
	wt, err := repo.Worktree()
	if err != nil {
		return []checkResult{{level: levelFail, name: "Git", message: fmt.Sprintf("worktree: %v", err)}}
	}
	st, err := wt.Status()
	if err != nil {
		return []checkResult{{level: levelFail, name: "Git", message: fmt.Sprintf("status: %v", err)}}
	}
	if st.IsClean() {
		results = append(results, checkResult{level: levelOK, name: "Git", message: "clean"})
	} else {
		results = append(results, checkResult{
			level:   levelWarn,
			name:    "Git",
			message: fmt.Sprintf("dirty (%d файл(ов) изменены)", len(st)),
		})
	}
	if cfg.Storage.Git.Remote != "" {
		remote, err := repo.Remote("origin")
		if err != nil {
			results = append(results, checkResult{
				level:   levelWarn,
				name:    "Git remote",
				message: "origin не настроен",
			})
		} else {
			actual := ""
			if urls := remote.Config().URLs; len(urls) > 0 {
				actual = urls[0]
			}
			masked := maskDSN(actual)
			if actual != cfg.Storage.Git.Remote {
				results = append(results, checkResult{
					level:   levelWarn,
					name:    "Git remote",
					message: fmt.Sprintf("URL mismatch: %s != %s", masked, maskDSN(cfg.Storage.Git.Remote)),
				})
			} else {
				results = append(results, checkResult{
					level:   levelOK,
					name:    "Git remote",
					message: fmt.Sprintf("origin → %s", masked),
				})
			}
		}
	}
	return results
}

func checkEditor() checkResult {
	for _, env := range []string{"VISUAL", "EDITOR"} {
		if v := os.Getenv(env); v != "" {
			return checkResult{level: levelOK, name: "Редактор", message: fmt.Sprintf("$%s=%s", env, v)}
		}
	}
	return checkResult{level: levelWarn, name: "Редактор", message: "VISUAL/EDITOR не заданы, по умолчанию vim"}
}
