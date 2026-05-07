package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jtprogru/jtpost/internal/adapters/fsrepo"
	"github.com/jtprogru/jtpost/internal/core"
)

func resetListFlags() {
	listStatuses = nil
	listTags = nil
	listSearch = ""
	listFormat = "table"
	listNoID = false
}

func TestCLIList_FormatJSON_NonEmpty(t *testing.T) {
	dir := t.TempDir()
	configPath := writeTestConfig(t, dir, dir)

	repo, err := fsrepo.NewFileSystemRepository(dir)
	if err != nil {
		t.Fatal(err)
	}
	post := &core.Post{
		ID:      mustParsePostID("list-json-1"),
		Title:   "JSON Post",
		Slug:    "json-post",
		Status:  core.StatusDraft,
		Content: "content",
	}
	fillTestPostDefaults(post)
	if err := repo.Create(core.WithTenant(context.Background(), testTenant1), post); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(resetListFlags)
	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"list", "-c", configPath, "-f", "json"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("list failed: %v\nstderr: %s", err, errBuf.String())
	}
	rootCmd.SetOut(nil)
	rootCmd.SetErr(nil)

	out := outBuf.String()
	if strings.TrimSpace(out) == "[]" {
		t.Fatalf("expected non-empty JSON array, got: %q", out)
	}
	var posts []map[string]any
	if err := json.Unmarshal([]byte(out), &posts); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out)
	}
	if len(posts) != 1 {
		t.Errorf("expected 1 post in JSON, got %d", len(posts))
	}
}

func TestCLIList_FormatJSON_Empty(t *testing.T) {
	dir := t.TempDir()
	configPath := writeTestConfig(t, dir, dir)

	t.Cleanup(resetListFlags)
	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"list", "-c", configPath, "-f", "json"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("list failed: %v\nstderr: %s", err, errBuf.String())
	}
	rootCmd.SetOut(nil)
	rootCmd.SetErr(nil)

	got := strings.TrimSpace(outBuf.String())
	if got != "[]" {
		t.Errorf("expected '[]', got: %q", got)
	}
}
