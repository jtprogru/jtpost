package cli

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/adapters/config"
)

// fakeUUIDGen возвращает заранее заданную последовательность UUID. Используется
// для тестов поведения init при коллизиях префиксов.
type fakeUUIDGen struct {
	seq []uuid.UUID
	pos int
}

func (g *fakeUUIDGen) New() uuid.UUID {
	if g.pos >= len(g.seq) {
		// fallback to deterministic UUID, чтобы тест не падал.
		return uuid.New()
	}
	u := g.seq[g.pos]
	g.pos++
	return u
}

// withTempDir переключается в новую временную директорию и возвращает функцию для отката.
// Это нужно потому, что init создаёт posts_dir/templates_dir относительно cwd.
func withTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	return dir
}

func runInit(t *testing.T, args []string, stdin string) (stderr string, err error) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetIn(strings.NewReader(stdin))
	rootCmd.SetArgs(args)
	err = rootCmd.Execute()
	rootCmd.SetIn(nil)
	rootCmd.SetOut(nil)
	rootCmd.SetErr(nil)
	return errBuf.String(), err
}

func TestCLIInit_NoExistingFile_CreatesAll(t *testing.T) {
	dir := withTempDir(t)
	cfgPath := filepath.Join(dir, ".jtpost.yaml")

	_, err := runInit(t, []string{"init", "-c", cfgPath}, "")
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// Файл конфига создан.
	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load failed: %v", err)
	}
	if cfg.Auth.TenantDefault == uuid.Nil {
		t.Error("tenant_default should be non-zero")
	}
	if cfg.Auth.AuthorDefault == uuid.Nil {
		t.Error("author_default should be non-zero")
	}

	// Директория templates_dir создана.
	if info, err := os.Stat(cfg.TemplatesDir); err != nil || !info.IsDir() {
		t.Errorf("templates dir not created: %v", err)
	}
	// Директория posts_dir/<tenant_short>/ создана.
	short := shortID(cfg.Auth.TenantDefault)
	tenantDir := filepath.Join(cfg.PostsDir, short)
	if info, err := os.Stat(tenantDir); err != nil || !info.IsDir() {
		t.Errorf("tenant posts dir %q not created: %v", tenantDir, err)
	}
}

func TestCLIInit_ExistingFile_AnswerNo_FilePreserved(t *testing.T) {
	dir := withTempDir(t)
	cfgPath := filepath.Join(dir, ".jtpost.yaml")

	original := []byte("posts_dir: original\n")
	if err := os.WriteFile(cfgPath, original, 0o644); err != nil {
		t.Fatal(err)
	}
	hashBefore := sha256.Sum256(original)

	errOut, err := runInit(t, []string{"init", "-c", cfgPath}, "n\n")
	if err != nil {
		t.Fatalf("init returned error: %v", err)
	}
	if !strings.Contains(errOut, "Aborted") {
		t.Errorf("expected 'Aborted' on stderr, got: %q", errOut)
	}

	got, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	hashAfter := sha256.Sum256(got)
	if hashBefore != hashAfter {
		t.Error("config file changed despite 'n' answer")
	}
}

func TestCLIInit_ExistingFile_AnswerYes_Overwritten(t *testing.T) {
	dir := withTempDir(t)
	cfgPath := filepath.Join(dir, ".jtpost.yaml")

	if err := os.WriteFile(cfgPath, []byte("posts_dir: old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := runInit(t, []string{"init", "-c", cfgPath}, "y\n")
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load failed: %v", err)
	}
	if cfg.Auth.TenantDefault == uuid.Nil {
		t.Error("tenant_default should be non-zero after overwrite")
	}
}

func TestCLIInit_ForceFlag_NoPrompt(t *testing.T) {
	dir := withTempDir(t)
	cfgPath := filepath.Join(dir, ".jtpost.yaml")

	if err := os.WriteFile(cfgPath, []byte("posts_dir: old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Stdin пустой — без --force зависнет/вернёт Aborted; с --force должно перезаписать.
	_, err := runInit(t, []string{"init", "-c", cfgPath, "--force"}, "")
	if err != nil {
		t.Fatalf("init --force failed: %v", err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load failed: %v", err)
	}
	if cfg.Auth.TenantDefault == uuid.Nil {
		t.Error("tenant_default should be set after --force overwrite")
	}
}

func TestCLIInit_UUIDPrefixUniqueness(t *testing.T) {
	dir := withTempDir(t)
	cfgPath := filepath.Join(dir, ".jtpost.yaml")

	// Конструируем последовательность UUID:
	// 1) tenant с префиксом "deadbeef..."
	// 2) author 1 — тот же префикс (коллизия)
	// 3) author 2 — другой префикс (успех)
	tenant := uuid.MustParse("deadbeef-0000-7000-8000-000000000001")
	collide := uuid.MustParse("deadbeef-0000-7000-8000-000000000002")
	unique := uuid.MustParse("01900000-0000-7000-8000-00000000000A")

	saved := initUUIDGen
	t.Cleanup(func() { initUUIDGen = saved })
	initUUIDGen = &fakeUUIDGen{seq: []uuid.UUID{tenant, collide, unique}}

	_, err := runInit(t, []string{"init", "-c", cfgPath}, "")
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load failed: %v", err)
	}
	if cfg.Auth.TenantDefault != tenant {
		t.Errorf("tenant_default = %v, want %v", cfg.Auth.TenantDefault, tenant)
	}
	if cfg.Auth.AuthorDefault != unique {
		t.Errorf("author_default = %v, want %v (после регенерации из-за коллизии)", cfg.Auth.AuthorDefault, unique)
	}
	if shortID(cfg.Auth.TenantDefault) == shortID(cfg.Auth.AuthorDefault) {
		t.Error("tenant и author имеют одинаковый short id")
	}
}
