package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/adapters/storage"
	"github.com/jtprogru/jtpost/internal/core"
)

// mkMigrateCfg возвращает Config с двумя backend'ами готовыми к открытию:
// Storage.Type=fs (default), но настроены и sqlite/postgres DSN-ы.
func mkMigrateCfg(t *testing.T) *config.Config {
	t.Helper()
	cfg := config.NewDefaultConfig()
	cfg.PostsDir = filepath.Join(t.TempDir(), "posts")
	cfg.Storage.SQLite.DSN = filepath.Join(t.TempDir(), "x.db")
	cfg.Auth.TenantDefault = uuid.MustParse("01900000-0000-7000-8000-000000000001")
	cfg.Auth.AuthorDefault = uuid.MustParse("01900000-0000-7000-8000-000000000002")
	return cfg
}

func TestIsValidStorageType(t *testing.T) {
	for _, st := range []string{"fs", "sqlite", "postgres"} {
		if !isValidStorageType(st) {
			t.Errorf("isValidStorageType(%q) = false, want true", st)
		}
	}
	for _, st := range []string{"", "mysql", "mongo"} {
		if isValidStorageType(st) {
			t.Errorf("isValidStorageType(%q) = true, want false", st)
		}
	}
}

// TestMigrate_FSToSQLite_Roundtrip — full round-trip:
// создаём пост в fs-репо → запускаем "миграцию" руками через storage.OpenAs → проверяем в sqlite.
func TestMigrate_FSToSQLite_Roundtrip(t *testing.T) {
	cfg := mkMigrateCfg(t)
	tenantID := cfg.Auth.TenantDefault
	authorID := cfg.Auth.AuthorDefault

	// Источник fs.
	srcRepo, srcCloser, err := storage.OpenAs(cfg, "fs")
	if err != nil {
		t.Fatalf("open fs: %v", err)
	}
	defer srcCloser.Close()

	ctx := scopeContext(context.Background(), tenantID, authorID)

	post := &core.Post{
		ID:        core.GeneratePostID("", core.SystemClock{}.Now()),
		TenantID:  tenantID,
		AuthorID:  authorID,
		Title:     "Migrate me",
		Slug:      "migrate-me",
		Status:    core.StatusDraft,
		Content:   "body",
		Revision:  1,
		CreatedAt: core.SystemClock{}.Now(),
		UpdatedAt: core.SystemClock{}.Now(),
	}
	if err := srcRepo.Create(ctx, post); err != nil {
		t.Fatalf("fs Create: %v", err)
	}

	// Получаем все посты из fs.
	posts, err := srcRepo.List(ctx, core.PostFilter{TenantID: tenantID})
	if err != nil {
		t.Fatalf("fs List: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("fs got %d posts, want 1", len(posts))
	}

	// Открываем target sqlite.
	dstRepo, dstCloser, err := storage.OpenAs(cfg, "sqlite")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer dstCloser.Close()

	mig, ok := dstRepo.(core.MigratableRepository)
	if !ok {
		t.Fatal("sqlite repo must implement MigratableRepository")
	}
	if err := mig.ImportPosts(ctx, posts); err != nil {
		t.Fatalf("ImportPosts: %v", err)
	}

	count, err := mig.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 1 {
		t.Errorf("sqlite count=%d, want 1", count)
	}
}

// TestMigrate_LegacyDBFlag — устаревший --db флаг должен экзитить через 2.
// Проверяем через cobra (запуск migrateCmd с args).
func TestMigrate_LegacyDBFlag_Detected(t *testing.T) {
	// Невозможно надёжно протестировать os.Exit(2) без подмены exit-функции.
	// Вместо этого проверим, что флаг определён, скрыт и помечен как deprecated-style:
	flag := migrateCmd.Flags().Lookup("db")
	if flag == nil {
		t.Fatal("--db flag must be registered (for legacy detection)")
	}
	if !flag.Hidden {
		t.Error("--db flag should be hidden")
	}
}

// TestMigrate_Cmd_RequiresFlags — запуск migrateCmd без --from/--to → ошибка.
func TestMigrate_Cmd_RequiresFlags(t *testing.T) {
	// reset module-level flags
	migrateFrom = ""
	migrateTo = ""

	buf := &bytes.Buffer{}
	migrateCmd.SetOut(buf)
	migrateCmd.SetErr(buf)
	migrateCmd.SetArgs([]string{})
	err := migrateCmd.RunE(migrateCmd, []string{})
	if err == nil {
		t.Fatal("expected error when --from/--to missing")
	}
}

// TestMigrate_Cmd_SameSourceTarget — --from X --to X → ошибка.
func TestMigrate_Cmd_SameSourceTarget(t *testing.T) {
	migrateFrom = "fs"
	migrateTo = "fs"
	defer func() { migrateFrom, migrateTo = "", "" }()

	err := migrateCmd.RunE(migrateCmd, []string{})
	if err == nil || !contains(err.Error(), "differ") {
		t.Fatalf("expected 'must differ' error, got %v", err)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || (len(s) > 0 && len(sub) > 0 && indexOf(s, sub) >= 0))
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
