package fsrepo

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
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
	if !strings.Contains(result.Content, "This is just content") {
		t.Errorf("Content = %q", result.Content)
	}
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		wantErr bool
	}{
		{"time.Time", time.Now(), false},
		{"RFC3339 string", "2026-03-15T10:00:00Z", false},
		{"date string", "2026-03-15", false},
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
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := IsValidPostStatus(tt.status); got != tt.want {
				t.Errorf("IsValidPostStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFrontmatter_RoundTrip_AllFields(t *testing.T) {
	// Property: CP-6
	now := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	excerpt := "Short excerpt"
	revisionSHA := "abcdef0123"
	deadline := now.Add(24 * time.Hour)

	cover := &core.Attachment{
		ID:       uuid.New(),
		Type:     core.AttachmentTypePhoto,
		Path:     "media/cover.png",
		Caption:  "Cover image",
		MimeType: "image/png",
		Size:     1024,
	}
	atts := []core.Attachment{
		{
			ID:       uuid.New(),
			Type:     core.AttachmentTypeDocument,
			Path:     "media/doc.pdf",
			MimeType: "application/pdf",
			Size:     2048,
		},
		{
			ID:   uuid.New(),
			Type: core.AttachmentTypeVideo,
			URL:  "https://example.com/video.mp4",
		},
	}
	history := []core.PublishAttempt{
		{
			ID:              uuid.New(),
			At:              now.Add(-3 * time.Hour),
			Target:          "telegram",
			Status:          "success",
			MessageID:       "msg-1",
			ResponsePayload: json.RawMessage(`{"ok":true}`),
			RetryAttempt:    1,
			Duration:        100 * time.Millisecond,
		},
		{
			ID:           uuid.New(),
			At:           now.Add(-2 * time.Hour),
			Target:       "telegram",
			Status:       "failed",
			Error:        "boom",
			RetryAttempt: 2,
			Duration:     50 * time.Millisecond,
		},
		{
			ID:           uuid.New(),
			At:           now.Add(-1 * time.Hour),
			Target:       "telegram",
			Status:       "success",
			MessageID:    "msg-2",
			RetryAttempt: 3,
			Duration:     75 * time.Millisecond,
		},
	}

	post := &core.Post{
		ID:             mustParsePostID("rt-all-fields"),
		TenantID:       testTenant1,
		AuthorID:       testAuthor1,
		Title:          "Round-trip Post",
		Slug:           "round-trip-post",
		Status:         core.StatusReady,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       3,
		Tags:           []string{"go", "test"},
		Deadline:       &deadline,
		Excerpt:        &excerpt,
		CoverImage:     cover,
		Attachments:    atts,
		PublishHistory: history,
		RevisionSHA:    &revisionSHA,
		Content:        "Body content here.",
	}

	data, err := SerializePost(post)
	if err != nil {
		t.Fatalf("SerializePost() error = %v", err)
	}

	parsed, err := ParsePost(data)
	if err != nil {
		t.Fatalf("ParsePost() error = %v", err)
	}

	if parsed.ID != post.ID {
		t.Errorf("ID mismatch: %v vs %v", parsed.ID, post.ID)
	}
	if parsed.TenantID != post.TenantID {
		t.Errorf("TenantID mismatch")
	}
	if parsed.AuthorID != post.AuthorID {
		t.Errorf("AuthorID mismatch")
	}
	if parsed.Revision != 3 {
		t.Errorf("Revision = %d, want 3", parsed.Revision)
	}
	if parsed.Excerpt == nil || *parsed.Excerpt != excerpt {
		t.Errorf("Excerpt mismatch")
	}
	if parsed.RevisionSHA == nil || *parsed.RevisionSHA != revisionSHA {
		t.Errorf("RevisionSHA mismatch")
	}
	if parsed.CoverImage == nil || parsed.CoverImage.Path != cover.Path {
		t.Errorf("CoverImage mismatch")
	}
	if len(parsed.Attachments) != 2 {
		t.Errorf("Attachments len = %d, want 2", len(parsed.Attachments))
	}
	if len(parsed.PublishHistory) != 3 {
		t.Errorf("PublishHistory len = %d, want 3", len(parsed.PublishHistory))
	}
	if parsed.Content != "Body content here." {
		t.Errorf("Content mismatch: %q", parsed.Content)
	}
}

func TestFrontmatter_PublishHistoryTruncation(t *testing.T) {
	// Property: CP-5
	now := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	attempts := make([]core.PublishAttempt, 15)
	for i := range 15 {
		attempts[i] = core.PublishAttempt{
			ID:           uuid.New(),
			At:           now.Add(time.Duration(i) * time.Hour),
			Target:       "telegram",
			Status:       "success",
			RetryAttempt: i + 1,
		}
	}

	post := &core.Post{
		ID:             mustParsePostID("trunc-1"),
		TenantID:       testTenant1,
		AuthorID:       testAuthor1,
		Title:          "Trunc",
		Slug:           "trunc",
		Status:         core.StatusReady,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
		PublishHistory: attempts,
		Content:        "x",
	}

	data, err := SerializePost(post)
	if err != nil {
		t.Fatalf("SerializePost() error = %v", err)
	}
	parsed, err := ParsePost(data)
	if err != nil {
		t.Fatalf("ParsePost() error = %v", err)
	}

	if len(parsed.PublishHistory) != 10 {
		t.Fatalf("PublishHistory len = %d, want 10", len(parsed.PublishHistory))
	}
	// Должны остаться записи с RetryAttempt 6..15 (последние 10 по At desc).
	// После SerializePost они в desc-порядке, при ParsePost остаются в том же порядке.
	first := parsed.PublishHistory[0]
	last := parsed.PublishHistory[9]
	if first.RetryAttempt != 15 {
		t.Errorf("first RetryAttempt = %d, want 15", first.RetryAttempt)
	}
	if last.RetryAttempt != 6 {
		t.Errorf("last RetryAttempt = %d, want 6", last.RetryAttempt)
	}
}

func TestFrontmatter_RejectsMissingFields(t *testing.T) {
	// Property: CP-7
	now := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	base := &core.Post{
		ID:        mustParsePostID("base"),
		TenantID:  testTenant1,
		AuthorID:  testAuthor1,
		Title:     "Title",
		Slug:      "slug",
		Status:    core.StatusDraft,
		CreatedAt: now,
		UpdatedAt: now,
		Revision:  1,
		Content:   "content",
	}

	tests := []struct {
		name   string
		mutate func(p *core.Post)
	}{
		{"missing id", func(p *core.Post) { p.ID = core.PostID{} }},
		{"missing tenant_id", func(p *core.Post) { p.TenantID = uuid.Nil }},
		{"missing author_id", func(p *core.Post) { p.AuthorID = uuid.Nil }},
		{"missing title", func(p *core.Post) { p.Title = "" }},
		{"missing slug", func(p *core.Post) { p.Slug = "" }},
		{"missing status", func(p *core.Post) { p.Status = "" }},
		{"missing created_at", func(p *core.Post) { p.CreatedAt = time.Time{} }},
		{"missing updated_at", func(p *core.Post) { p.UpdatedAt = time.Time{} }},
		{"missing revision", func(p *core.Post) { p.Revision = 0 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cp := *base
			tt.mutate(&cp)
			data, err := SerializePost(&cp)
			if err != nil {
				t.Fatalf("SerializePost() error = %v", err)
			}
			_, err = ParsePost(data)
			if !errors.Is(err, core.ErrValidation) {
				t.Errorf("ParsePost(%s) error = %v, want ErrValidation", tt.name, err)
			}
		})
	}
}

func TestAttachment_AbsolutePath_RejectsTraversal(t *testing.T) {
	postsDir := t.TempDir()
	att := core.Attachment{Path: "../etc/passwd"}
	_, err := att.AbsolutePath(postsDir)
	if !errors.Is(err, core.ErrValidation) {
		t.Errorf("AbsolutePath(traversal) error = %v, want ErrValidation", err)
	}

	att2 := core.Attachment{Path: "media/safe.png"}
	got, err := att2.AbsolutePath(postsDir)
	if err != nil {
		t.Errorf("AbsolutePath(safe) error = %v", err)
	}
	if !strings.HasPrefix(got, postsDir) {
		t.Errorf("AbsolutePath(safe) = %s, want prefix %s", got, postsDir)
	}
}
