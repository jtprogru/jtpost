// Package storage предоставляет единый конструктор core.PostRepository
// по cfg.Storage.Type ∈ {fs, sqlite, postgres}.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/adapters/fsrepo"
	"github.com/jtprogru/jtpost/internal/adapters/postgres"
	"github.com/jtprogru/jtpost/internal/adapters/sqlite"
	"github.com/jtprogru/jtpost/internal/core"
)

type nopCloser struct{}

func (nopCloser) Close() error { return nil }

// Open возвращает PostRepository по cfg.Storage.Type. При неуспехе
// соединения/миграции возвращает ошибку обёрнутую в core.ErrConfigInvalid
// или core.ErrMigrationFailed.
func Open(cfg *config.Config) (core.PostRepository, io.Closer, error) {
	return OpenAs(cfg, cfg.Storage.Type)
}

// OpenAs позволяет вызывающему коду переопределить storageType (используется
// командой `jtpost migrate --from --to`).
func OpenAs(cfg *config.Config, storageType string) (core.PostRepository, io.Closer, error) {
	switch storageType {
	case "", "fs":
		repo, err := fsrepo.NewFileSystemRepository(cfg.PostsDir)
		if err != nil {
			return nil, nil, err
		}
		return repo, nopCloser{}, nil
	case "sqlite":
		dsn := cfg.SQLiteDSN()
		if dsn == "" {
			return nil, nil, errors.Join(core.ErrConfigInvalid, errors.New("storage.sqlite.dsn required"))
		}
		repo, err := sqlite.NewSQLitePostRepository(sqlite.Config{DSN: dsn})
		if err != nil {
			return nil, nil, err
		}
		return repo, repo, nil
	case "postgres":
		if cfg.Storage.Postgres.DSN == "" {
			return nil, nil, errors.Join(core.ErrConfigInvalid, errors.New("storage.postgres.dsn required"))
		}
		repo, err := postgres.NewPostgresRepository(context.Background(), postgres.Config{
			DSN:             cfg.Storage.Postgres.DSN,
			MaxOpenConns:    cfg.Storage.Postgres.MaxOpenConns,
			MaxIdleConns:    cfg.Storage.Postgres.MaxIdleConns,
			ConnMaxLifetime: cfg.Storage.Postgres.ConnMaxLifetime,
		})
		if err != nil {
			return nil, nil, err
		}
		return repo, repo, nil
	default:
		return nil, nil, fmt.Errorf("%w: unknown storage.type %q", core.ErrConfigInvalid, storageType)
	}
}
