package core

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// TestPost_TenantShortID проверяет длину 8 и совпадение с первыми 8 hex-символами.
//
// Property: CP-8 (TenantShortIDPrefix).
func TestPost_TenantShortID(t *testing.T) {
	tests := []struct {
		name   string
		tenant string
		expect string
	}{
		{"v7 1", "01900000-0000-7000-8000-000000000001", "01900000"},
		{"v7 2", "deadbeef-cafe-7000-8000-000000000000", "deadbeef"},
		{"all-zero", "00000000-0000-0000-0000-000000000000", "00000000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := uuid.MustParse(tt.tenant)
			p := Post{TenantID: id}
			got := p.TenantShortID()
			if len(got) != 8 {
				t.Errorf("len(TenantShortID()) = %d, want 8", len(got))
			}
			if got != tt.expect {
				t.Errorf("TenantShortID() = %q, want %q", got, tt.expect)
			}
			// Проверка алгоритма: совпадение с первыми 8 символами без дефисов.
			expected := strings.ReplaceAll(id.String(), "-", "")[:8]
			if got != expected {
				t.Errorf("TenantShortID() = %q, prefix-stripped = %q", got, expected)
			}
		})
	}
}

// TestPostFilter_TenantShortID проверяет, что метод доступен также на PostFilter.
func TestPostFilter_TenantShortID(t *testing.T) {
	id := uuid.MustParse("01900000-0000-7000-8000-000000000001")
	f := PostFilter{TenantID: id}
	if got := f.TenantShortID(); got != "01900000" {
		t.Errorf("PostFilter.TenantShortID() = %q, want 01900000", got)
	}
}

// TestAttachmentType_Validate проверяет принятие/отклонение значений типа.
//
// Property: CP-7 (FrontmatterRequiredFields style validation).
func TestAttachmentType_Validate(t *testing.T) {
	tests := []struct {
		t       AttachmentType
		wantErr bool
	}{
		{AttachmentTypePhoto, false},
		{AttachmentTypeVideo, false},
		{AttachmentTypeDocument, false},
		{AttachmentTypeAudio, false},
		{AttachmentType(""), true},
		{AttachmentType("image"), true},
		{AttachmentType("file"), true},
	}
	for _, tt := range tests {
		t.Run(string(tt.t), func(t *testing.T) {
			err := tt.t.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(%q) err=%v, wantErr=%v", string(tt.t), err, tt.wantErr)
			}
		})
	}
}

// TestPostFilter_IsValidSortKey проверяет whitelist ключей сортировки.
//
// Property: CP-11 (SortByWhitelist).
func TestPostFilter_IsValidSortKey(t *testing.T) {
	valid := []string{"created_at", "updated_at", "deadline", "scheduled_at", "title", "status"}
	invalid := []string{"", "id", "tenant_id", "RANDOM", "tags"}
	for _, k := range valid {
		if !IsValidSortKey(k) {
			t.Errorf("IsValidSortKey(%q) = false, want true", k)
		}
	}
	for _, k := range invalid {
		if IsValidSortKey(k) {
			t.Errorf("IsValidSortKey(%q) = true, want false", k)
		}
	}
}

// fixedPost возвращает Post со всеми обязательными и почти всеми опциональными
// полями. Используется в round-trip тестах.
func fixedPost(t *testing.T) Post {
	t.Helper()
	tenant := uuid.MustParse("01900000-0000-7000-8000-000000000001")
	author := uuid.MustParse("01900000-0000-7000-8000-000000000002")
	att1ID := uuid.MustParse("01900000-0000-7000-8000-000000000010")
	att2ID := uuid.MustParse("01900000-0000-7000-8000-000000000011")
	coverID := uuid.MustParse("01900000-0000-7000-8000-000000000012")
	pa1ID := uuid.MustParse("01900000-0000-7000-8000-000000000020")
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	updated := time.Date(2026, 1, 2, 3, 5, 0, 0, time.UTC)
	excerpt := "Excerpt text"
	revSHA := "abcdef0123456789"

	return Post{
		ID:        PostID(uuid.MustParse("01900000-0000-7000-8000-000000000003")),
		TenantID:  tenant,
		AuthorID:  author,
		Title:     "Test Title",
		Slug:      "test-title",
		Status:    StatusDraft,
		CreatedAt: created,
		UpdatedAt: updated,
		Revision:  3,
		Tags:      []string{"go", "test"},
		Excerpt:   &excerpt,
		CoverImage: &Attachment{
			ID:       coverID,
			Type:     AttachmentTypePhoto,
			Path:     "covers/cover.png",
			MimeType: "image/png",
			Size:     12345,
		},
		Attachments: []Attachment{
			{ID: att1ID, Type: AttachmentTypePhoto, Path: "media/1.png", MimeType: "image/png", Size: 100},
			{ID: att2ID, Type: AttachmentTypeVideo, URL: "https://cdn.example/v.mp4", MimeType: "video/mp4", Size: 999999},
		},
		PublishHistory: []PublishAttempt{
			{
				ID:              pa1ID,
				At:              updated,
				Target:          "telegram",
				Status:          "success",
				MessageID:       "42",
				ResponsePayload: json.RawMessage(`{"ok":true}`),
				RetryAttempt:    1,
				Duration:        250 * time.Millisecond,
			},
		},
		RevisionSHA: &revSHA,
		External:    ExternalLinks{TelegramURL: "https://t.me/x/42"},
	}
}

// TestPost_RoundTrip_YAML проверяет round-trip Post через YAML.
//
// Property: CP-6 (PostRoundTrip).
func TestPost_RoundTrip_YAML(t *testing.T) {
	original := fixedPost(t)

	data, err := yaml.Marshal(&original)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}

	var decoded Post
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID mismatch")
	}
	if decoded.TenantID != original.TenantID {
		t.Errorf("TenantID mismatch")
	}
	if decoded.AuthorID != original.AuthorID {
		t.Errorf("AuthorID mismatch")
	}
	if decoded.Revision != original.Revision {
		t.Errorf("Revision %d != %d", decoded.Revision, original.Revision)
	}
	if !decoded.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("CreatedAt mismatch: %v != %v", decoded.CreatedAt, original.CreatedAt)
	}
	if decoded.Excerpt == nil || *decoded.Excerpt != *original.Excerpt {
		t.Errorf("Excerpt mismatch")
	}
	if decoded.CoverImage == nil || decoded.CoverImage.ID != original.CoverImage.ID {
		t.Errorf("CoverImage mismatch")
	}
	if len(decoded.Attachments) != len(original.Attachments) {
		t.Errorf("Attachments len %d != %d", len(decoded.Attachments), len(original.Attachments))
	}
	if len(decoded.PublishHistory) != len(original.PublishHistory) {
		t.Errorf("PublishHistory len %d != %d", len(decoded.PublishHistory), len(original.PublishHistory))
	}
	if decoded.RevisionSHA == nil || *decoded.RevisionSHA != *original.RevisionSHA {
		t.Errorf("RevisionSHA mismatch")
	}
}

// TestPost_RoundTrip_JSON проверяет round-trip Post через JSON.
//
// Property: CP-6 (PostRoundTrip).
func TestPost_RoundTrip_JSON(t *testing.T) {
	original := fixedPost(t)
	original.Content = "Body markdown"

	data, err := json.Marshal(&original)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var decoded Post
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if decoded.TenantID != original.TenantID {
		t.Errorf("TenantID mismatch")
	}
	if decoded.Revision != original.Revision {
		t.Errorf("Revision %d != %d", decoded.Revision, original.Revision)
	}
	if decoded.Content != original.Content {
		t.Errorf("Content mismatch")
	}
}

// TestPublishAttempt_RoundTrip_YAML проверяет round-trip PublishAttempt с
// inline JSON в ResponsePayload.
//
// Property: CP-6.
func TestPublishAttempt_RoundTrip_YAML(t *testing.T) {
	original := PublishAttempt{
		ID:              uuid.MustParse("01900000-0000-7000-8000-000000000020"),
		At:              time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC),
		Target:          "telegram",
		Status:          "failed",
		Error:           "rate limited",
		ResponsePayload: json.RawMessage(`{"error_code":429}`),
		RetryAttempt:    2,
		Duration:        500 * time.Millisecond,
	}

	data, err := yaml.Marshal(&original)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	var decoded PublishAttempt
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	if decoded.ID != original.ID {
		t.Errorf("ID mismatch")
	}
	if decoded.Status != original.Status {
		t.Errorf("Status mismatch")
	}
	if decoded.RetryAttempt != original.RetryAttempt {
		t.Errorf("RetryAttempt mismatch")
	}
	if decoded.Duration != original.Duration {
		t.Errorf("Duration mismatch")
	}
}
