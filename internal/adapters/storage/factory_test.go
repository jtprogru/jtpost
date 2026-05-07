package storage

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/adapters/fsrepo"
	"github.com/jtprogru/jtpost/internal/adapters/gitrepo"
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

func TestOpen_Dispatch_FS_GitEnabled(t *testing.T) {
	cfg := mkCfg(t, func(c *config.Config) {
		c.Storage.Type = "fs"
		c.Storage.Git.Enabled = true
		c.Storage.Git.Branch = "main"
	})
	repo, closer, err := Open(cfg)
	if err != nil {
		t.Fatalf("Open(fs+git): %v", err)
	}
	defer closer.Close()
	if _, ok := repo.(*gitrepo.GitDecorator); !ok {
		t.Fatalf("expected *gitrepo.GitDecorator, got %T", repo)
	}
}

func TestOpen_Dispatch_FS_GitDisabled_Unchanged(t *testing.T) {
	cfg := mkCfg(t, func(c *config.Config) {
		c.Storage.Type = "fs"
		c.Storage.Git.Enabled = false
	})
	repo, closer, err := Open(cfg)
	if err != nil {
		t.Fatalf("Open(fs): %v", err)
	}
	defer closer.Close()
	if _, ok := repo.(*fsrepo.FileSystemPostRepository); !ok {
		t.Fatalf("expected fsrepo (no decorator), got %T", repo)
	}
}

func TestOpenBundle_FS_NoUsersTokens(t *testing.T) {
	cfg := mkCfg(t, func(c *config.Config) { c.Storage.Type = "fs" })
	b, err := OpenBundle(cfg)
	if err != nil {
		t.Fatalf("OpenBundle(fs): %v", err)
	}
	defer b.Closer.Close()
	if b.Posts == nil {
		t.Error("Posts must be non-nil")
	}
	if b.Users != nil {
		t.Errorf("Users must be nil for fs, got %T", b.Users)
	}
	if b.Tokens != nil {
		t.Errorf("Tokens must be nil for fs, got %T", b.Tokens)
	}
}

func TestOpenBundle_SQLite_AllReposNonNil(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "x.db")
	cfg := mkCfg(t, func(c *config.Config) {
		c.Storage.Type = "sqlite"
		c.Storage.SQLite.DSN = dbPath
	})
	b, err := OpenBundle(cfg)
	if err != nil {
		t.Fatalf("OpenBundle(sqlite): %v", err)
	}
	defer b.Closer.Close()
	if b.Posts == nil || b.Users == nil || b.Tokens == nil {
		t.Errorf("all repos must be non-nil for sqlite, got %v / %v / %v", b.Posts == nil, b.Users == nil, b.Tokens == nil)
	}
}

func TestOpenBundle_InvalidType(t *testing.T) {
	cfg := mkCfg(t, func(c *config.Config) { c.Storage.Type = "mongo" })
	_, err := OpenBundle(cfg)
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
