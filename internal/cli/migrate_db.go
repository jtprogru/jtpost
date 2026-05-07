package cli

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"strings"

	"github.com/jtprogru/jtpost/internal/adapters/config"
	postgresadapter "github.com/jtprogru/jtpost/internal/adapters/postgres"
	sqliteadapter "github.com/jtprogru/jtpost/internal/adapters/sqlite"
	"github.com/pressly/goose/v3"
	"github.com/spf13/cobra"

	_ "github.com/jackc/pgx/v5/stdlib" // pgx-stdlib для goose+postgres
	_ "modernc.org/sqlite"             // sqlite-driver
)

var migrateDBTo string

var migrateDBCmd = &cobra.Command{
	Use:   "db",
	Short: "Управление схемой БД (goose-миграции)",
	Long:  `Применяет/проверяет статус миграций для выбранного backend (sqlite или postgres).`,
}

var migrateDBUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Применить все pending-миграции",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGoose(cmd.Context(), cmd, "up")
	},
}

var migrateDBStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Показать статус миграций",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGoose(cmd.Context(), cmd, "status")
	},
}

func init() {
	migrateDBCmd.PersistentFlags().StringVar(&migrateDBTo, "to", "", "backend (sqlite|postgres)")
	_ = migrateDBCmd.MarkPersistentFlagRequired("to")

	migrateDBCmd.AddCommand(migrateDBUpCmd)
	migrateDBCmd.AddCommand(migrateDBStatusCmd)
	migrateCmd.AddCommand(migrateDBCmd)
}

func runGoose(ctx context.Context, cmd *cobra.Command, action string) error {
	if migrateDBTo != "sqlite" && migrateDBTo != "postgres" {
		return fmt.Errorf("--to must be sqlite or postgres, got %q", migrateDBTo)
	}

	configPath, _ := cmd.Flags().GetString("config")
	cfg, err := loadConfigForMigrate(configPath)
	if err != nil {
		return err
	}

	driver, dsn, dialect, migrationsFS, err := gooseTarget(cfg, migrateDBTo)
	if err != nil {
		return err
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return fmt.Errorf("sql.Open(%s): %w", driver, err)
	}
	defer db.Close()

	goose.SetBaseFS(migrationsFS)
	defer goose.SetBaseFS(nil)

	if err := goose.SetDialect(dialect); err != nil {
		return fmt.Errorf("goose.SetDialect: %w", err)
	}

	switch action {
	case "up":
		if err := goose.UpContext(ctx, db, "migrations"); err != nil {
			return fmt.Errorf("goose up: %w", err)
		}
		v, err := goose.GetDBVersionContext(ctx, db)
		if err != nil {
			return fmt.Errorf("goose version: %w", err)
		}
		fmt.Printf("✅ Миграции применены. Текущая версия: %d\n", v)
	case "status":
		if err := goose.StatusContext(ctx, db, "migrations"); err != nil {
			return fmt.Errorf("goose status: %w", err)
		}
	default:
		return fmt.Errorf("unknown action: %s", action)
	}
	return nil
}

func gooseTarget(cfg *config.Config, backend string) (driver, dsn, dialect string, migrationsFS fs.FS, err error) {
	switch backend {
	case "sqlite":
		dsn = cfg.SQLiteDSN()
		if dsn == "" {
			return "", "", "", nil, fmt.Errorf("storage.sqlite.dsn required")
		}
		return "sqlite", dsn, "sqlite3", sqliteadapter.MigrationsFS(), nil
	case "postgres":
		dsn = cfg.Storage.Postgres.DSN
		if dsn == "" {
			return "", "", "", nil, fmt.Errorf("storage.postgres.dsn required")
		}
		return "pgx", dsn, "postgres", postgresadapter.MigrationsFS(), nil
	default:
		return "", "", "", nil, fmt.Errorf("unsupported backend: %s", backend)
	}
}

// maskDSN скрывает пароль в DSN для безопасного логирования.
// Используется в doctor.go.
func maskDSN(dsn string) string {
	// postgres://user:pass@host/db → postgres://user:***@host/db
	scheme, rest, ok := strings.Cut(dsn, "://")
	if !ok {
		return dsn
	}
	creds, host, ok := strings.Cut(rest, "@")
	if !ok {
		return dsn
	}
	user, _, ok := strings.Cut(creds, ":")
	if !ok {
		return dsn
	}
	return scheme + "://" + user + ":***@" + host
}
