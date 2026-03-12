package fsrepo

import (
	"strings"
	"testing"
	"time"

	"github.com/jtprogru/jtpost/internal/core"
)

func TestParseFrontmatter_WithYAML(t *testing.T) {
	content := `---
title: Test Post
slug: test-post
status: draft
tags:
  - go
  - test
---

This is the content.
`

	result, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatalf("ParseFrontmatter() error = %v", err)
	}

	if !result.HasFrontmatter {
		t.Error("HasFrontmatter = false, want true")
	}
	if result.Type != FrontmatterYAML {
		t.Errorf("Type = %v, want FrontmatterYAML", result.Type)
	}
	if result.Content != "This is the content." {
		t.Errorf("Content = %q, want 'This is the content.'", result.Content)
	}

	metadata := result.Metadata
	if metadata["title"] != "Test Post" {
		t.Errorf("title = %v, want 'Test Post'", metadata["title"])
	}
	if metadata["slug"] != "test-post" {
		t.Errorf("slug = %v, want 'test-post'", metadata["slug"])
	}
}

func TestParseFrontmatter_WithoutFrontmatter(t *testing.T) {
	content := `This is just content without frontmatter.

No YAML here.
`

	result, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatalf("ParseFrontmatter() error = %v", err)
	}

	if result.HasFrontmatter {
		t.Error("HasFrontmatter = true, want false")
	}
	if result.Type != FrontmatterNone {
		t.Errorf("Type = %v, want FrontmatterNone", result.Type)
	}
	if !strings.Contains(result.Content, "This is just content") {
		t.Errorf("Content = %q", result.Content)
	}
}

func TestParseFrontmatter_UnclosedFrontmatter(t *testing.T) {
	content := `---
title: Test Post
slug: test-post

This is content.
`

	result, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatalf("ParseFrontmatter() error = %v", err)
	}

	// Непарный frontmatter должен считаться отсутствием
	if result.HasFrontmatter {
		t.Error("HasFrontmatter = true, want false (unclosed frontmatter)")
	}
}

func TestNormalizeFrontmatter_NoFrontmatter(t *testing.T) {
	result := &FrontmatterResult{
		Type:           FrontmatterNone,
		HasFrontmatter: false,
		Content:        "Just content",
		Metadata:       make(map[string]interface{}),
	}

	post, err := NormalizeFrontmatter(result, "my-slug")
	if err != nil {
		t.Fatalf("NormalizeFrontmatter() error = %v", err)
	}

	if post.Slug != "my-slug" {
		t.Errorf("Slug = %v, want my-slug", post.Slug)
	}
	if post.Status != core.StatusIdea {
		t.Errorf("Status = %v, want idea", post.Status)
	}
	if len(post.Platforms) != 1 || post.Platforms[0] != core.PlatformTelegram {
		t.Errorf("Platforms = %v, want [telegram]", post.Platforms)
	}
	if post.Content != "Just content" {
		t.Errorf("Content = %q, want 'Just content'", post.Content)
	}
}

func TestNormalizeFrontmatter_WithYAML(t *testing.T) {
	metadata := map[string]interface{}{
		"title":   "My Title",
		"slug":    "my-slug",
		"status":  "ready",
		"tags":    []interface{}{"go", "cli"},
		"deadline": "2026-03-15",
	}

	result := &FrontmatterResult{
		Type:           FrontmatterYAML,
		HasFrontmatter: true,
		Content:        "Content here",
		Metadata:       metadata,
	}

	post, err := NormalizeFrontmatter(result, "default-slug")
	if err != nil {
		t.Fatalf("NormalizeFrontmatter() error = %v", err)
	}

	if post.Title != "My Title" {
		t.Errorf("Title = %v, want 'My Title'", post.Title)
	}
	if post.Slug != "my-slug" {
		t.Errorf("Slug = %v, want 'my-slug'", post.Slug)
	}
	if post.Status != core.StatusReady {
		t.Errorf("Status = %v, want ready", post.Status)
	}
	if len(post.Tags) != 2 {
		t.Errorf("Tags length = %v, want 2", len(post.Tags))
	}
	if post.Deadline == nil {
		t.Error("Deadline = nil, want date")
	}
}

func TestNormalizeFrontmatter_InvalidStatus(t *testing.T) {
	metadata := map[string]interface{}{
		"title":  "Test",
		"slug":   "test",
		"status": "invalid_status",
	}

	result := &FrontmatterResult{
		Type:           FrontmatterYAML,
		HasFrontmatter: true,
		Content:        "Content",
		Metadata:       metadata,
	}

	post, err := NormalizeFrontmatter(result, "test")
	if err != nil {
		t.Fatalf("NormalizeFrontmatter() error = %v", err)
	}

	// Должен установиться в draft
	if post.Status != core.StatusDraft {
		t.Errorf("Status = %v, want draft (for invalid status)", post.Status)
	}
}

func TestNormalizeFrontmatter_Platforms(t *testing.T) {
	tests := []struct {
		name         string
		platformsRaw interface{}
		wantLength   int
		wantFirst    string
	}{
		{
			name:         "array of platforms",
			platformsRaw: []interface{}{"telegram", "blog"},
			wantLength:   2,
			wantFirst:    "telegram",
		},
		{
			name:         "single platform string",
			platformsRaw: "telegram",
			wantLength:   1,
			wantFirst:    "telegram",
		},
		{
			name:         "empty array",
			platformsRaw: []interface{}{},
			wantLength:   1,
			wantFirst:    "telegram", // default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := map[string]interface{}{
				"title":     "Test",
				"slug":      "test",
				"platforms": tt.platformsRaw,
			}

			result := &FrontmatterResult{
				Type:           FrontmatterYAML,
				HasFrontmatter: true,
				Content:        "Content",
				Metadata:       metadata,
			}

			post, err := NormalizeFrontmatter(result, "test")
			if err != nil {
				t.Fatalf("NormalizeFrontmatter() error = %v", err)
			}

			if len(post.Platforms) != tt.wantLength {
				t.Errorf("Platforms length = %v, want %v", len(post.Platforms), tt.wantLength)
			}
			if tt.wantLength > 0 && string(post.Platforms[0]) != tt.wantFirst {
				t.Errorf("First platform = %v, want %v", post.Platforms[0], tt.wantFirst)
			}
		})
	}
}

func TestNormalizeFrontmatter_Tags(t *testing.T) {
	tests := []struct {
		name      string
		tagsRaw   interface{}
		wantCount int
	}{
		{
			name:      "array of tags",
			tagsRaw:   []interface{}{"go", "cli", "test"},
			wantCount: 3,
		},
		{
			name:      "comma-separated string",
			tagsRaw:   "go, cli, test",
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := map[string]interface{}{
				"title": "Test",
				"slug":  "test",
				"tags":  tt.tagsRaw,
			}

			result := &FrontmatterResult{
				Type:           FrontmatterYAML,
				HasFrontmatter: true,
				Content:        "Content",
				Metadata:       metadata,
			}

			post, err := NormalizeFrontmatter(result, "test")
			if err != nil {
				t.Fatalf("NormalizeFrontmatter() error = %v", err)
			}

			if len(post.Tags) != tt.wantCount {
				t.Errorf("Tags length = %v, want %v", len(post.Tags), tt.wantCount)
			}
		})
	}
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
	}{
		{"time.Time", time.Now(), false},
		{"RFC3339 string", "2026-03-15T10:00:00Z", false},
		{"date string", "2026-03-15", false},
		{"datetime string", "2026-03-15 10:00:00", false},
		{"invalid string", "not a date", true},
		{"invalid type", 12345, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseTime(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTime() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsValidPostStatus(t *testing.T) {
	tests := []struct {
		status core.PostStatus
		want   bool
	}{
		{core.StatusIdea, true},
		{core.StatusDraft, true},
		{core.StatusReady, true},
		{core.StatusScheduled, true},
		{core.StatusPublished, true},
		{core.PostStatus("invalid"), false},
		{core.PostStatus(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := IsValidPostStatus(tt.status); got != tt.want {
				t.Errorf("IsValidPostStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildFrontmatter(t *testing.T) {
	now := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	
	post := &core.Post{
		ID:          "test-id-123",
		Title:       "Test Post",
		Slug:        "test-post",
		Status:      core.StatusDraft,
		Platforms:   []core.Platform{core.PlatformTelegram},
		Tags:        []string{"go", "test"},
		Deadline:    &now,
		Content:     "Test content",
		External: core.ExternalLinks{
			TelegramURL: "https://t.me/test",
		},
	}

	frontmatter, err := BuildFrontmatter(post)
	if err != nil {
		t.Fatalf("BuildFrontmatter() error = %v", err)
	}

	// Проверяем наличие ключевых полей
	if !strings.Contains(frontmatter, `id: "test-id-123"`) {
		t.Error("Frontmatter missing id")
	}
	if !strings.Contains(frontmatter, `title: "Test Post"`) {
		t.Error("Frontmatter missing title")
	}
	if !strings.Contains(frontmatter, `status: "draft"`) {
		t.Error("Frontmatter missing status")
	}
	if !strings.Contains(frontmatter, "telegram") {
		t.Error("Frontmatter missing telegram platform")
	}
	if !strings.Contains(frontmatter, "go") {
		t.Error("Frontmatter missing go tag")
	}
}

func TestSerializePostWithFrontmatter(t *testing.T) {
	post := &core.Post{
		ID:        "test-id",
		Title:     "Test",
		Slug:      "test",
		Status:    core.StatusDraft,
		Platforms: []core.Platform{core.PlatformTelegram},
		Tags:      []string{"test"},
		Content:   "Test content here",
	}

	data, err := SerializePostWithFrontmatter(post)
	if err != nil {
		t.Fatalf("SerializePostWithFrontmatter() error = %v", err)
	}

	dataStr := string(data)
	if !strings.HasPrefix(dataStr, "---") {
		t.Error("Serialized data should start with ---")
	}
	if !strings.Contains(dataStr, "Test content here") {
		t.Error("Serialized data missing content")
	}
}

func TestNormalizeFrontmatter_ExternalLinks(t *testing.T) {
	metadata := map[string]interface{}{
		"title": "Test",
		"slug":  "test",
		"external": map[string]interface{}{
			"telegram_url": "https://t.me/test_post",
			"blog_url":     "https://example.com/blog", // Должен игнорироваться
		},
	}

	result := &FrontmatterResult{
		Type:           FrontmatterYAML,
		HasFrontmatter: true,
		Content:        "Content",
		Metadata:       metadata,
	}

	post, err := NormalizeFrontmatter(result, "test")
	if err != nil {
		t.Fatalf("NormalizeFrontmatter() error = %v", err)
	}

	if post.External.TelegramURL != "https://t.me/test_post" {
		t.Errorf("TelegramURL = %v, want https://t.me/test_post", post.External.TelegramURL)
	}
	// Blog URL не должен сохраняться
}
