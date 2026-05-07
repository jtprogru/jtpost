package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// runCmd запускает rootCmd с заданными аргументами и возвращает err.
func runCmd(t *testing.T, args []string) error {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	rootCmd.SetOut(nil)
	rootCmd.SetErr(nil)
	return err
}

func resetNewFlags() {
	newTenant = ""
	newAuthor = ""
	newSlug = ""
	newTags = nil
	newEditor = ""
}

func TestCLINew_DefaultsFromConfig(t *testing.T) {
	dir := t.TempDir()
	postsDir := filepath.Join(dir, "posts")
	configPath := writeTestConfig(t, dir, postsDir)

	t.Setenv("EDITOR", "true")
	t.Setenv("VISUAL", "true")
	t.Cleanup(resetNewFlags)

	err := runCmd(t, []string{"new", "-c", configPath, "--slug", "test-slug", "Test Post"})
	if err != nil {
		t.Fatalf("new failed: %v", err)
	}

	short := shortID(testTenant1)
	expected := filepath.Join(postsDir, short, "test-slug.md")
	if _, statErr := os.Stat(expected); statErr != nil {
		t.Errorf("expected file at %s: %v", expected, statErr)
	}
}

func TestCLINew_TenantFlag_Overrides(t *testing.T) {
	dir := t.TempDir()
	postsDir := filepath.Join(dir, "posts")
	configPath := writeTestConfig(t, dir, postsDir)

	t.Setenv("EDITOR", "true")
	t.Setenv("VISUAL", "true")
	t.Cleanup(resetNewFlags)

	override := uuid.MustParse("01900000-0000-7000-8000-0000000000FF")
	err := runCmd(t, []string{
		"new", "-c", configPath,
		"--slug", "with-tenant",
		"--tenant", override.String(),
		"With Tenant",
	})
	if err != nil {
		t.Fatalf("new --tenant failed: %v", err)
	}

	short := strings.ReplaceAll(override.String(), "-", "")[:8]
	expected := filepath.Join(postsDir, short, "with-tenant.md")
	if _, statErr := os.Stat(expected); statErr != nil {
		t.Errorf("expected file at %s, err: %v", expected, statErr)
	}
}

func TestCLINew_InvalidUUIDFlag(t *testing.T) {
	dir := t.TempDir()
	postsDir := filepath.Join(dir, "posts")
	configPath := writeTestConfig(t, dir, postsDir)

	t.Setenv("EDITOR", "true")
	t.Setenv("VISUAL", "true")
	t.Cleanup(resetNewFlags)

	err := runCmd(t, []string{
		"new", "-c", configPath,
		"--tenant", "not-a-uuid",
		"X",
	})
	if err == nil {
		t.Fatal("expected error for invalid --tenant UUID, got nil")
	}
	if !strings.Contains(err.Error(), "invalid UUID for --tenant") {
		t.Errorf("error %q does not mention 'invalid UUID for --tenant'", err.Error())
	}
}
