package gitrepo

import (
	"context"
	"testing"

	"github.com/jtprogru/jtpost/internal/adapters/config"
)

func TestGitDecorator_History_EmptyRepo_NoEntries(t *testing.T) {
	d, _ := newDecorator(t, config.GitStorageConfig{Enabled: true, AutoCommit: false})
	defer func() { _ = d.Close() }()

	post := mkPost("hello")
	entries, err := d.History(context.Background(), post, 0)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries on empty repo, got %d", len(entries))
	}
}

func TestGitDecorator_History_ReturnsCommitsForFile(t *testing.T) {
	d, _ := newDecorator(t, config.GitStorageConfig{
		Enabled: true, AutoCommit: true, CommitTemplate: "{{.Operation}}: {{.Slug}}",
	})
	defer func() { _ = d.Close() }()

	post := mkPost("hello")
	ctx := context.Background()
	if err := d.Create(ctx, post); err != nil {
		t.Fatalf("create: %v", err)
	}
	post.Title = "Updated"
	if err := d.Update(ctx, post); err != nil {
		t.Fatalf("update: %v", err)
	}

	entries, err := d.History(ctx, post, 0)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(entries) < 2 {
		t.Fatalf("expected ≥2 entries (create+update), got %d", len(entries))
	}
	if entries[0].ShortHash == "" || len(entries[0].ShortHash) != 8 {
		t.Errorf("ShortHash должен быть 8 символов: %q", entries[0].ShortHash)
	}
	if entries[0].Author == "" {
		t.Error("Author не должен быть пустым")
	}
	if entries[0].At.IsZero() {
		t.Error("At не должен быть zero")
	}
	// Свежий коммит — первый (commits идут от HEAD назад).
	if entries[0].At.Before(entries[len(entries)-1].At) {
		t.Errorf("entries должны быть в обратном хронологическом порядке")
	}
}

func TestGitDecorator_History_RespectsLimit(t *testing.T) {
	d, _ := newDecorator(t, config.GitStorageConfig{Enabled: true, AutoCommit: true})
	defer func() { _ = d.Close() }()

	post := mkPost("limit")
	ctx := context.Background()
	if err := d.Create(ctx, post); err != nil {
		t.Fatal(err)
	}
	for i := range 5 {
		post.Title = "v" + string(rune('0'+i))
		if err := d.Update(ctx, post); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := d.History(ctx, post, 3)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 entries (limit), got %d", len(entries))
	}
}

func TestGitDecorator_History_NilPost(t *testing.T) {
	d, _ := newDecorator(t, config.GitStorageConfig{Enabled: true})
	defer func() { _ = d.Close() }()
	if _, err := d.History(context.Background(), nil, 0); err == nil {
		t.Error("expected error for nil post")
	}
}
