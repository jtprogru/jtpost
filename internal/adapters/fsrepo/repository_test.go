package fsrepo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jtprogru/jtpost/internal/core"
)

func TestFileSystemPostRepository_CreateAndGetByID(t *testing.T) {
	dir := t.TempDir()

	repo, err := NewFileSystemRepository(dir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	now := time.Now()
	post := &core.Post{
		ID:        "test-id-123",
		Title:     "Test Post",
		Slug:      "test-post",
		Status:    core.StatusDraft,
		Platforms: []core.Platform{core.PlatformTelegram},
		Tags:      []string{"test", "go"},
		Deadline:  &now,
		Content:   "Test content",
	}

	ctx := context.Background()

	// Create
	if err := repo.Create(ctx, post); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// GetByID - ID теперь сохраняется в файле
	got, err := repo.GetByID(ctx, post.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if got.Title != post.Title {
		t.Errorf("Title = %v, want %v", got.Title, post.Title)
	}
	if got.Slug != post.Slug {
		t.Errorf("Slug = %v, want %v", got.Slug, post.Slug)
	}
	if got.Status != post.Status {
		t.Errorf("Status = %v, want %v", got.Status, post.Status)
	}
	if len(got.Tags) != len(post.Tags) {
		t.Errorf("Tags length = %v, want %v", len(got.Tags), len(post.Tags))
	}
}

func TestFileSystemPostRepository_List(t *testing.T) {
	dir := t.TempDir()

	repo, err := NewFileSystemRepository(dir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	ctx := context.Background()

	// Создаём несколько постов
	posts := []*core.Post{
		{ID: "1", Title: "Post 1", Slug: "post-1", Status: core.StatusDraft, Platforms: []core.Platform{core.PlatformTelegram}, Tags: []string{"go"}},
		{ID: "2", Title: "Post 2", Slug: "post-2", Status: core.StatusReady, Platforms: []core.Platform{}, Tags: []string{"cli"}},
		{ID: "3", Title: "Post 3", Slug: "post-3", Status: core.StatusDraft, Platforms: []core.Platform{core.PlatformTelegram}, Tags: []string{"go", "test"}},
	}

	for _, p := range posts {
		if err := repo.Create(ctx, p); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	tests := []struct {
		name       string
		filter     core.PostFilter
		wantLength int
	}{
		{"no filter", core.PostFilter{}, 3},
		{"filter by status draft", core.PostFilter{Statuses: []core.PostStatus{core.StatusDraft}}, 2},
		{"filter by status ready", core.PostFilter{Statuses: []core.PostStatus{core.StatusReady}}, 1},
		{"filter by platform telegram", core.PostFilter{Platforms: []core.Platform{core.PlatformTelegram}}, 2},
		{"filter by tag go", core.PostFilter{Tags: []string{"go"}}, 2},
		{"filter by search", core.PostFilter{Search: "Post 1"}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.List(ctx, tt.filter)
			if err != nil {
				t.Errorf("List() error = %v", err)
				return
			}
			if len(got) != tt.wantLength {
				t.Errorf("List() length = %v, want %v", len(got), tt.wantLength)
			}
		})
	}
}

func TestFileSystemPostRepository_Update(t *testing.T) {
	dir := t.TempDir()

	repo, err := NewFileSystemRepository(dir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	ctx := context.Background()

	post := &core.Post{
		ID:        "test-id",
		Title:     "Original Title",
		Slug:      "original-title",
		Status:    core.StatusIdea,
		Platforms: []core.Platform{core.PlatformTelegram},
		Content:   "Original content",
	}

	if err := repo.Create(ctx, post); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Update
	post.Title = "Updated Title"
	post.Status = core.StatusDraft
	post.Content = "Updated content"

	if err := repo.Update(ctx, post); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Verify
	got, err := repo.GetByID(ctx, post.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if got.Title != "Updated Title" {
		t.Errorf("Title = %v, want Updated Title", got.Title)
	}
	if got.Status != core.StatusDraft {
		t.Errorf("Status = %v, want draft", got.Status)
	}
	if got.Content != "Updated content" {
		t.Errorf("Content = %v, want Updated content", got.Content)
	}
}

func TestFileSystemPostRepository_Delete(t *testing.T) {
	dir := t.TempDir()

	repo, err := NewFileSystemRepository(dir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	ctx := context.Background()

	post := &core.Post{
		ID:        "test-id",
		Title:     "To Delete",
		Slug:      "to-delete",
		Status:    core.StatusIdea,
		Platforms: []core.Platform{core.PlatformTelegram},
		Content:   "Content",
	}

	if err := repo.Create(ctx, post); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Delete
	if err := repo.Delete(ctx, post.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify deleted - второй Delete должен вернуть ErrNotFound
	err = repo.Delete(ctx, post.ID)
	if err == nil {
		t.Errorf("Delete() после удаления expected error, got nil")
	} else if !errors.Is(err, core.ErrNotFound) {
		t.Errorf("Delete() после удаления error = %v, want ErrNotFound", err)
	}

	_, err = repo.GetByID(ctx, post.ID)
	if err == nil {
		t.Errorf("GetByID() after delete expected error, got nil")
	} else if !errors.Is(err, core.ErrNotFound) {
		t.Errorf("GetByID() after delete error = %v, want ErrNotFound", err)
	}
}

func TestFileSystemPostRepository_GetByID_NotFound(t *testing.T) {
	dir := t.TempDir()

	repo, err := NewFileSystemRepository(dir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	ctx := context.Background()

	_, err = repo.GetByID(ctx, "non-existent-id")
	if err == nil {
		t.Errorf("GetByID() error = nil, want ErrNotFound")
	} else if !errors.Is(err, core.ErrNotFound) {
		t.Errorf("GetByID() error = %v, want ErrNotFound", err)
	}
}

func TestFileSystemPostRepository_Create_AlreadyExists(t *testing.T) {
	dir := t.TempDir()

	repo, err := NewFileSystemRepository(dir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	ctx := context.Background()

	post := &core.Post{
		ID:        "test-id",
		Title:     "Test",
		Slug:      "test",
		Status:    core.StatusIdea,
		Platforms: []core.Platform{core.PlatformTelegram},
		Content:   "Content",
	}

	if err := repo.Create(ctx, post); err != nil {
		t.Fatalf("Create() first error = %v", err)
	}

	// Try to create again
	if err := repo.Create(ctx, post); !errors.Is(err, core.ErrAlreadyExists) {
		t.Errorf("Create() second error = %v, want ErrAlreadyExists", err)
	}
}

func TestParsePost(t *testing.T) {
	data := []byte(`---
title: Test Post
slug: test-post
status: draft
platforms:
  - telegram
tags:
  - go
  - test
external:
  telegram_url: ""
---

This is the post content.
`)

	post, err := ParsePost(data)
	if err != nil {
		t.Fatalf("ParsePost() error = %v", err)
	}

	if post.Title != "Test Post" {
		t.Errorf("Title = %v, want Test Post", post.Title)
	}
	if post.Slug != "test-post" {
		t.Errorf("Slug = %v, want test-post", post.Slug)
	}
	if post.Status != core.StatusDraft {
		t.Errorf("Status = %v, want draft", post.Status)
	}
	if len(post.Platforms) != 1 || post.Platforms[0] != core.PlatformTelegram {
		t.Errorf("Platforms = %v, want [telegram]", post.Platforms)
	}
	if len(post.Tags) != 2 {
		t.Errorf("Tags length = %v, want 2", len(post.Tags))
	}
	if post.Content != "This is the post content." {
		t.Errorf("Content = %v, want 'This is the post content.'", post.Content)
	}
}

func TestSerializePost(t *testing.T) {
	now := time.Now()
	post := &core.Post{
		ID:          "test-id",
		Title:       "Test Post",
		Slug:        "test-post",
		Status:      core.StatusDraft,
		Platforms:   []core.Platform{core.PlatformTelegram, core.PlatformTelegram},
		Tags:        []string{"go", "test"},
		Deadline:    &now,
		ScheduledAt: &now,
		Content:     "Test content here",
		External: core.ExternalLinks{
			TelegramURL: "https://t.me/post",
		},
	}

	data, err := SerializePost(post)
	if err != nil {
		t.Fatalf("SerializePost() error = %v", err)
	}

	// Parse back and verify
	parsed, err := ParsePost(data)
	if err != nil {
		t.Fatalf("ParsePost() after serialize error = %v", err)
	}

	if parsed.Title != post.Title {
		t.Errorf("Title = %v, want %v", parsed.Title, post.Title)
	}
	if parsed.Status != post.Status {
		t.Errorf("Status = %v, want %v", parsed.Status, post.Status)
	}
	if parsed.Content != post.Content {
		t.Errorf("Content = %v, want %v", parsed.Content, post.Content)
	}
}

func TestNewFileSystemRepository_CreatesDirectory(t *testing.T) {
	tempDir := t.TempDir()
	newDir := filepath.Join(tempDir, "new-posts")

	repo, err := NewFileSystemRepository(newDir)
	if err != nil {
		t.Fatalf("NewFileSystemRepository() error = %v", err)
	}

	if repo == nil {
		t.Fatal("NewFileSystemRepository() returned nil")
	}

	// Verify directory was created
	if _, err := os.Stat(newDir); os.IsNotExist(err) {
		t.Errorf("Directory was not created")
	}
}
