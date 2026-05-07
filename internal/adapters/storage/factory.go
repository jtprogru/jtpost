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
	"github.com/jtprogru/jtpost/internal/adapters/gitrepo"
	"github.com/jtprogru/jtpost/internal/adapters/postgres"
	"github.com/jtprogru/jtpost/internal/adapters/sqlite"
	"github.com/jtprogru/jtpost/internal/core"
)

type nopCloser struct{}

func (nopCloser) Close() error { return nil }

// Bundle объединяет все репозитории, открытые поверх одного storage backend.
// Для fs-режима Users и Tokens равны nil (FS не поддерживает users).
type Bundle struct {
	Posts         core.PostRepository
	Users         core.UserRepository         // nil for fs
	Tokens        core.TokenRepository        // nil for fs
	Sessions      core.SessionRepository      // nil for fs (and nil during F4b T-2 in-progress)
	OAuthAccounts core.OAuthAccountRepository // nil for fs
	Outbox        core.OutboxRepository       // nil for fs
	AuditLog      core.AuditRepository        // nil for fs
	Closer        io.Closer
}

// OpenBundle возвращает Bundle по cfg.Storage.Type.
func OpenBundle(cfg *config.Config) (*Bundle, error) {
	switch cfg.Storage.Type {
	case "", "fs":
		repo, err := fsrepo.NewFileSystemRepository(cfg.PostsDir)
		if err != nil {
			return nil, err
		}
		closer := io.Closer(nopCloser{})
		var posts core.PostRepository = repo
		if cfg.Storage.Git.Enabled {
			dec, err := gitrepo.NewGitDecorator(repo, cfg.PostsDir, cfg.Storage.Git)
			if err != nil {
				return nil, err
			}
			posts = dec
			closer = dec
		}
		return &Bundle{Posts: posts, Users: nil, Tokens: nil, Closer: closer}, nil
	case "sqlite":
		dsn := cfg.SQLiteDSN()
		if dsn == "" {
			return nil, errors.Join(core.ErrConfigInvalid, errors.New("storage.sqlite.dsn required"))
		}
		repo, err := sqlite.NewSQLitePostRepository(sqlite.Config{DSN: dsn})
		if err != nil {
			return nil, err
		}
		return &Bundle{Posts: repo, Users: repo.Users(), Tokens: repo.Tokens(), Sessions: repo.Sessions(), OAuthAccounts: repo.OAuthAccounts(), Outbox: repo.Outbox(), AuditLog: repo.AuditLog(), Closer: repo}, nil
	case "postgres":
		if cfg.Storage.Postgres.DSN == "" {
			return nil, errors.Join(core.ErrConfigInvalid, errors.New("storage.postgres.dsn required"))
		}
		repo, err := postgres.NewPostgresRepository(context.Background(), postgres.Config{
			DSN:             cfg.Storage.Postgres.DSN,
			MaxOpenConns:    cfg.Storage.Postgres.MaxOpenConns,
			MaxIdleConns:    cfg.Storage.Postgres.MaxIdleConns,
			ConnMaxLifetime: cfg.Storage.Postgres.ConnMaxLifetime,
		})
		if err != nil {
			return nil, err
		}
		return &Bundle{Posts: repo, Users: repo.Users(), Tokens: repo.Tokens(), Sessions: repo.Sessions(), OAuthAccounts: repo.OAuthAccounts(), Outbox: repo.Outbox(), AuditLog: repo.AuditLog(), Closer: repo}, nil
	default:
		return nil, fmt.Errorf("%w: unknown storage.type %q", core.ErrConfigInvalid, cfg.Storage.Type)
	}
}

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
		if cfg.Storage.Git.Enabled {
			dec, err := gitrepo.NewGitDecorator(repo, cfg.PostsDir, cfg.Storage.Git)
			if err != nil {
				return nil, nil, err
			}
			return dec, dec, nil
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
