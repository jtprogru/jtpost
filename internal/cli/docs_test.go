package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDocs_GeneratesMarkdownTree(t *testing.T) {
	// Не parallel: shared docsCmd flag state.
dir := t.TempDir()
	if err := docsCmd.Flags().Set("out", dir); err != nil {
		t.Fatalf("set flag: %v", err)
	}
	t.Cleanup(func() { _ = docsCmd.Flags().Set("out", "./docs/cli") })

	if err := runDocs(docsCmd, nil); err != nil {
		t.Fatalf("runDocs: %v", err)
	}

	for _, want := range []string{"jtpost.md", "jtpost_list.md", "jtpost_publish.md"} {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("missing %s: %v", want, err)
		}
	}
	// Ровно один subcommand для smoke-check содержимого.
	body, err := os.ReadFile(filepath.Join(dir, "jtpost.md"))
	if err != nil {
		t.Fatalf("read root md: %v", err)
	}
	if len(body) < 200 {
		t.Errorf("root doc looks too small: %d bytes", len(body))
	}
}

func TestDocs_CreatesMissingDir(t *testing.T) {
	// Не parallel: shared docsCmd flag state.
dir := filepath.Join(t.TempDir(), "nested", "docs")
	if err := docsCmd.Flags().Set("out", dir); err != nil {
		t.Fatalf("set flag: %v", err)
	}
	t.Cleanup(func() { _ = docsCmd.Flags().Set("out", "./docs/cli") })

	if err := runDocs(docsCmd, nil); err != nil {
		t.Fatalf("runDocs: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("dir not created: %v", err)
	}
}
