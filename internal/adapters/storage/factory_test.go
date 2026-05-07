package storage

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/adapters/fsrepo"
	"github.com/jtprogru/jtpost/internal/adapters/sqlite"
	"github.com/jtprogru/jtpost/internal/core"
)

func mkCfg(t *testing.T, mutate func(*config.Config)) *config.Config {
	t.Helper()
	cfg := config.NewDefaultConfig()
	cfg.PostsDir = filepath.Join(t.TempDir(), "posts")
	cfg.Auth.TenantDefault = uuid.MustParse("01900000-0000-7000-8000-000000000001")
	cfg.Auth.AuthorDefault = uuid.MustParse("01900000-0000-7000-8000-000000000002")
	if mutate != nil {
		mutate(cfg)
	}
	return cfg
}

func TestOpen_Dispatch_FS(t *testing.T) {
	cfg := mkCfg(t, func(c *config.Config) { c.Storage.Type = "fs" })
	repo, closer, err := Open(cfg)
	if err != nil {
		t.Fatalf("Open(fs): %v", err)
	}
	defer closer.Close()
	if _, ok := repo.(*fsrepo.FileSystemPostRepository); !ok {
		t.Fatalf("expected *fsrepo.FileSystemPostRepository, got %T", repo)
	}
}

func TestOpen_Dispatch_EmptyType_DefaultsFS(t *testing.T) {
	cfg := mkCfg(t, func(c *config.Config) { c.Storage.Type = "" })
	repo, closer, err := Open(cfg)
	if err != nil {
		t.Fatalf("Open(\"\"): %v", err)
	}
	defer closer.Close()
	if _, ok := repo.(*fsrepo.FileSystemPostRepository); !ok {
		t.Fatalf("empty Type must default to fs, got %T", repo)
	}
}

func TestOpen_Dispatch_SQLite(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "x.db")
	cfg := mkCfg(t, func(c *config.Config) {
		c.Storage.Type = "sqlite"
		c.Storage.SQLite.DSN = dbPath
	})
	repo, closer, err := Open(cfg)
	if err != nil {
		t.Fatalf("Open(sqlite): %v", err)
	}
	defer closer.Close()
	if _, ok := repo.(*sqlite.PostRepository); !ok {
		t.Fatalf("expected *sqlite.PostRepository, got %T", repo)
	}
}

func TestOpen_Dispatch_SQLite_LegacyDSNFallback(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy.db")
	cfg := mkCfg(t, func(c *config.Config) {
		c.Storage.Type = "sqlite"
		c.Storage.SQLite.DSN = ""
		c.SQLite.DSN = dbPath
	})
	_, closer, err := Open(cfg)
	if err != nil {
		t.Fatalf("Open(sqlite legacy): %v", err)
	}
	closer.Close()
}

func TestOpen_InvalidType(t *testing.T) {
	cfg := mkCfg(t, func(c *config.Config) { c.Storage.Type = "mysql" })
	_, _, err := Open(cfg)
	if !errors.Is(err, core.ErrConfigInvalid) {
		t.Fatalf("want ErrConfigInvalid, got %v", err)
	}
}

func TestOpen_SQLite_MissingDSN(t *testing.T) {
	cfg := mkCfg(t, func(c *config.Config) {
		c.Storage.Type = "sqlite"
		c.Storage.SQLite.DSN = ""
		c.SQLite.DSN = ""
	})
	_, _, err := Open(cfg)
	if !errors.Is(err, core.ErrConfigInvalid) {
		t.Fatalf("want ErrConfigInvalid, got %v", err)
	}
}

func TestOpen_Postgres_MissingDSN(t *testing.T) {
	cfg := mkCfg(t, func(c *config.Config) {
		c.Storage.Type = "postgres"
		c.Storage.Postgres.DSN = ""
	})
	_, _, err := Open(cfg)
	if !errors.Is(err, core.ErrConfigInvalid) {
		t.Fatalf("want ErrConfigInvalid, got %v", err)
	}
}

func TestOpenAs_OverridesType(t *testing.T) {
	cfg := mkCfg(t, func(c *config.Config) { c.Storage.Type = "postgres" })
	repo, closer, err := OpenAs(cfg, "fs")
	if err != nil {
		t.Fatalf("OpenAs(fs): %v", err)
	}
	defer closer.Close()
	if _, ok := repo.(*fsrepo.FileSystemPostRepository); !ok {
		t.Fatalf("OpenAs override failed: %T", repo)
	}
}
