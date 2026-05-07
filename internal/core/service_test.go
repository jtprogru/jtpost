package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// testTenant и testAuthor — фиксированные UUID для тестов.
//
//nolint:gochecknoglobals // shared test constants
var (
	testTenant  = uuid.MustParse("01900000-0000-7000-8000-000000000001")
	testTenant2 = uuid.MustParse("01900000-0000-7000-8000-000000000002")
	testAuthor  = uuid.MustParse("01900000-0000-7000-8000-000000000003")
)

// fakeClock — фиксированный clock для тестов.
type fakeClock struct {
	t time.Time
}

func (c *fakeClock) Now() time.Time { return c.t }

// advance продвигает время на dur.
func (c *fakeClock) advance(dur time.Duration) { c.t = c.t.Add(dur) }

func newFakeClock() *fakeClock {
	return &fakeClock{t: time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)}
}

// newServiceFixture создаёт сервис с пустым mockRepository.
func newServiceFixture(t *testing.T) (*PostService, *mockRepository, *fakeClock) {
	t.Helper()
	repo := &mockRepository{posts: make(map[PostID]*Post)}
	clk := newFakeClock()
	return NewPostService(repo, clk), repo, clk
}

func validInput() CreatePostInput {
	return CreatePostInput{
		TenantID: testTenant,
		AuthorID: testAuthor,
		Title:    "Hello",
		Tags:     []string{"go"},
	}
}

// TestService_CreatePost_RequiresTenantID — REQ-1.1.
func TestService_CreatePost_RequiresTenantID(t *testing.T) {
	svc, _, _ := newServiceFixture(t)
	in := validInput()
	in.TenantID = uuid.Nil
	_, err := svc.CreatePost(context.Background(), in)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}

// TestService_CreatePost_RequiresAuthorID — REQ-1.2.
func TestService_CreatePost_RequiresAuthorID(t *testing.T) {
	svc, _, _ := newServiceFixture(t)
	in := validInput()
	in.AuthorID = uuid.Nil
	_, err := svc.CreatePost(context.Background(), in)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}

// TestService_CreatePost_SetsAudit — REQ-1.3.
//
// Property: CP-3 (RevisionMonotonic — initial state).
func TestService_CreatePost_SetsAudit(t *testing.T) {
	svc, _, clk := newServiceFixture(t)
	post, err := svc.CreatePost(context.Background(), validInput())
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}
	if !post.CreatedAt.Equal(clk.t) {
		t.Errorf("CreatedAt = %v, want %v", post.CreatedAt, clk.t)
	}
	if !post.UpdatedAt.Equal(clk.t) {
		t.Errorf("UpdatedAt = %v, want %v", post.UpdatedAt, clk.t)
	}
	if post.Revision != 1 {
		t.Errorf("Revision = %d, want 1", post.Revision)
	}
	if post.TenantID != testTenant {
		t.Errorf("TenantID mismatch")
	}
	if post.AuthorID != testAuthor {
		t.Errorf("AuthorID mismatch")
	}
	if post.Status != StatusIdea {
		t.Errorf("Status = %s, want idea", post.Status)
	}
}

// TestService_UpdatePost_IncrementsRevision — REQ-1.4.
//
// Property: CP-3 (RevisionMonotonic).
func TestService_UpdatePost_IncrementsRevision(t *testing.T) {
	svc, _, clk := newServiceFixture(t)
	post, err := svc.CreatePost(context.Background(), validInput())
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}
	originalCreated := post.CreatedAt

	for i := 1; i <= 10; i++ {
		clk.advance(time.Minute)
		post.Title = "Updated"
		if err := svc.UpdatePost(context.Background(), post); err != nil {
			t.Fatalf("UpdatePost iteration %d: %v", i, err)
		}
	}
	if post.Revision != 11 {
		t.Errorf("Revision after 10 updates = %d, want 11", post.Revision)
	}
	if !post.CreatedAt.Equal(originalCreated) {
		t.Errorf("CreatedAt changed: %v vs %v", post.CreatedAt, originalCreated)
	}
	if !post.UpdatedAt.Equal(clk.t) {
		t.Errorf("UpdatedAt %v != clock %v", post.UpdatedAt, clk.t)
	}
}

// TestService_UpdatePost_TenantImmutable — REQ-1.5.
//
// Property: CP-2 (TenantImmutability).
func TestService_UpdatePost_TenantImmutable(t *testing.T) {
	svc, repo, _ := newServiceFixture(t)
	post, err := svc.CreatePost(context.Background(), validInput())
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}
	originalTenant := post.TenantID

	post.TenantID = testTenant2
	post.Title = "Updated"
	err = svc.UpdatePost(context.Background(), post)
	if !errors.Is(err, ErrTenantMismatch) {
		t.Errorf("expected ErrTenantMismatch, got %v", err)
	}
	stored := repo.posts[post.ID]
	if stored.TenantID != originalTenant {
		t.Errorf("stored TenantID changed to %v", stored.TenantID)
	}
}

// TestService_UpdateStatus_AllowedTransitions — REQ-3.3.
//
// Property: CP-4, CP-10.
func TestService_UpdateStatus_AllowedTransitions(t *testing.T) {
	allowed := []struct {
		from, to PostStatus
	}{
		{StatusIdea, StatusDraft},
		{StatusDraft, StatusReady},
		{StatusReady, StatusScheduled},
		{StatusReady, StatusPublished},
		{StatusScheduled, StatusPublished},
		{StatusScheduled, StatusReady},
		{StatusScheduled, StatusFailed},
		{StatusFailed, StatusReady},
		{StatusFailed, StatusArchived},
		{StatusPublished, StatusArchived},
	}
	for _, tc := range allowed {
		t.Run(string(tc.from)+"->"+string(tc.to), func(t *testing.T) {
			svc, repo, _ := newServiceFixture(t)
			id := PostID(uuid.New())
			repo.posts[id] = &Post{
				ID:       id,
				TenantID: testTenant,
				AuthorID: testAuthor,
				Title:    "T",
				Slug:     "t",
				Status:   tc.from,
				Revision: 1,
			}
			post, err := svc.UpdateStatus(context.Background(), id, tc.to)
			if err != nil {
				t.Fatalf("transition %s->%s failed: %v", tc.from, tc.to, err)
			}
			if post.Status != tc.to {
				t.Errorf("Status = %s, want %s", post.Status, tc.to)
			}
		})
	}
}

// TestService_UpdateStatus_DisallowedRejected — REQ-3.3.
//
// Property: CP-10 (UpdateStatusTransitionEnforcement).
func TestService_UpdateStatus_DisallowedRejected(t *testing.T) {
	svc, repo, _ := newServiceFixture(t)
	id := PostID(uuid.New())
	repo.posts[id] = &Post{
		ID: id, TenantID: testTenant, AuthorID: testAuthor,
		Title: "T", Slug: "t", Status: StatusIdea, Revision: 1,
	}
	// idea → ready запрещён (только idea→draft)
	_, err := svc.UpdateStatus(context.Background(), id, StatusReady)
	if !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("expected ErrInvalidTransition, got %v", err)
	}
	// Состояние не должно измениться
	if repo.posts[id].Status != StatusIdea {
		t.Errorf("status changed despite error")
	}
}

// TestService_UpdateStatus_PublishedSetsPublishedAt — REQ-3.4.
func TestService_UpdateStatus_PublishedSetsPublishedAt(t *testing.T) {
	svc, repo, clk := newServiceFixture(t)
	id := PostID(uuid.New())
	repo.posts[id] = &Post{
		ID: id, TenantID: testTenant, AuthorID: testAuthor,
		Title: "T", Slug: "t", Status: StatusReady, Revision: 1,
	}
	post, err := svc.UpdateStatus(context.Background(), id, StatusPublished)
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	if post.PublishedAt == nil {
		t.Fatalf("PublishedAt is nil")
	}
	if !post.PublishedAt.Equal(clk.t) {
		t.Errorf("PublishedAt = %v, want %v", *post.PublishedAt, clk.t)
	}
}

// TestService_MarkFailed_AppendsHistory — REQ-3.5.
func TestService_MarkFailed_AppendsHistory(t *testing.T) {
	svc, repo, _ := newServiceFixture(t)
	id := PostID(uuid.New())
	repo.posts[id] = &Post{
		ID: id, TenantID: testTenant, AuthorID: testAuthor,
		Title: "T", Slug: "t", Status: StatusScheduled, Revision: 1,
	}
	post, err := svc.MarkFailed(context.Background(), id, "rate limited")
	if err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	if post.Status != StatusFailed {
		t.Errorf("Status = %s, want failed", post.Status)
	}
	if len(post.PublishHistory) != 1 {
		t.Fatalf("PublishHistory len = %d, want 1", len(post.PublishHistory))
	}
	att := post.PublishHistory[0]
	if att.Status != "failed" || att.Error != "rate limited" {
		t.Errorf("attempt = %+v", att)
	}
}

// TestService_MarkFailed_FromIdea_Rejected — only from scheduled allowed.
func TestService_MarkFailed_FromIdea_Rejected(t *testing.T) {
	svc, repo, _ := newServiceFixture(t)
	id := PostID(uuid.New())
	repo.posts[id] = &Post{
		ID: id, TenantID: testTenant, AuthorID: testAuthor,
		Title: "T", Slug: "t", Status: StatusIdea, Revision: 1,
	}
	_, err := svc.MarkFailed(context.Background(), id, "x")
	if !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("expected ErrInvalidTransition, got %v", err)
	}
}

// TestService_Archive_FromPublished — success.
func TestService_Archive_FromPublished(t *testing.T) {
	svc, repo, _ := newServiceFixture(t)
	id := PostID(uuid.New())
	repo.posts[id] = &Post{
		ID: id, TenantID: testTenant, AuthorID: testAuthor,
		Title: "T", Slug: "t", Status: StatusPublished, Revision: 1,
	}
	post, err := svc.Archive(context.Background(), id)
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if post.Status != StatusArchived {
		t.Errorf("Status = %s, want archived", post.Status)
	}
}

// TestService_Archive_FromIdea_Rejected.
func TestService_Archive_FromIdea_Rejected(t *testing.T) {
	svc, repo, _ := newServiceFixture(t)
	id := PostID(uuid.New())
	repo.posts[id] = &Post{
		ID: id, TenantID: testTenant, AuthorID: testAuthor,
		Title: "T", Slug: "t", Status: StatusIdea, Revision: 1,
	}
	_, err := svc.Archive(context.Background(), id)
	if !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("expected ErrInvalidTransition, got %v", err)
	}
}

// TestService_ListPosts_RejectsInvalidSortBy — REQ-4.4.
//
// Property: CP-11.
func TestService_ListPosts_RejectsInvalidSortBy(t *testing.T) {
	svc, _, _ := newServiceFixture(t)
	_, err := svc.ListPosts(context.Background(), PostFilter{
		TenantID: testTenant,
		SortBy:   "random",
	})
	if !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}

// TestService_GenerateSlug — preserves existing slug logic.
func TestService_GenerateSlug(t *testing.T) {
	svc, _, _ := newServiceFixture(t)
	tests := []struct {
		title    string
		expected string
	}{
		{"Hello World", "hello-world"},
		{"Привет Мир", "privet-mir"},
	}
	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			if got := svc.generateSlug(tt.title); got != tt.expected {
				t.Errorf("generateSlug(%q) = %q, want %q", tt.title, got, tt.expected)
			}
		})
	}
}

// TestService_DeletePost.
func TestService_DeletePost(t *testing.T) {
	svc, repo, _ := newServiceFixture(t)
	id := PostID(uuid.New())
	repo.posts[id] = &Post{
		ID: id, TenantID: testTenant, AuthorID: testAuthor,
		Title: "T", Slug: "t", Status: StatusDraft, Revision: 1,
	}
	if err := svc.DeletePost(context.Background(), id); err != nil {
		t.Errorf("DeletePost: %v", err)
	}
	if _, ok := repo.posts[id]; ok {
		t.Errorf("post still exists after delete")
	}

	if err := svc.DeletePost(context.Background(), PostID(uuid.New())); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// TestService_GetStats.
func TestService_GetStats(t *testing.T) {
	svc, repo, _ := newServiceFixture(t)
	for i, st := range []PostStatus{StatusDraft, StatusDraft, StatusReady, StatusPublished} {
		id := PostID(uuid.New())
		_ = i
		repo.posts[id] = &Post{
			ID: id, TenantID: testTenant, AuthorID: testAuthor,
			Title: "T", Slug: "t", Status: st, Revision: 1,
			Tags: []string{"go"},
		}
	}
	stats, err := svc.GetStats(context.Background(), testTenant)
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.Total != 4 {
		t.Errorf("Total = %d, want 4", stats.Total)
	}
	if stats.ByStatus[StatusDraft] != 2 {
		t.Errorf("Draft count = %d, want 2", stats.ByStatus[StatusDraft])
	}
	if stats.ByTag["go"] != 4 {
		t.Errorf("go tag count = %d, want 4", stats.ByTag["go"])
	}
}

// TestService_GetNextPost — basic priority test.
func TestService_GetNextPost(t *testing.T) {
	svc, repo, clk := newServiceFixture(t)
	pastDeadline := clk.t.Add(-24 * time.Hour)
	id := PostID(uuid.New())
	repo.posts[id] = &Post{
		ID: id, TenantID: testTenant, AuthorID: testAuthor,
		Title: "Overdue", Slug: "overdue", Status: StatusDraft, Revision: 1,
		Deadline: &pastDeadline,
	}
	id2 := PostID(uuid.New())
	repo.posts[id2] = &Post{
		ID: id2, TenantID: testTenant, AuthorID: testAuthor,
		Title: "Idea", Slug: "idea", Status: StatusIdea, Revision: 1,
	}

	next, err := svc.GetNextPost(context.Background(), testTenant)
	if err != nil {
		t.Fatalf("GetNextPost: %v", err)
	}
	if next == nil || next.ID != id {
		t.Errorf("expected overdue post, got %v", next)
	}
}

// TestService_GetNextPost_Empty.
func TestService_GetNextPost_Empty(t *testing.T) {
	svc, _, _ := newServiceFixture(t)
	next, err := svc.GetNextPost(context.Background(), testTenant)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if next != nil {
		t.Errorf("expected nil, got %v", next)
	}
}

// mockRepository — тестовая реализация PostRepository, поддерживающая
// фильтрацию по TenantID.
type mockRepository struct {
	posts   map[PostID]*Post
	listErr error
}

func (m *mockRepository) GetByID(_ context.Context, id PostID) (*Post, error) {
	post, ok := m.posts[id]
	if !ok {
		return nil, ErrNotFound
	}
	// Возвращаем копию, чтобы тесты могли изменять возвращённый Post без
	// мутации хранимого состояния.
	cp := *post
	return &cp, nil
}

func (m *mockRepository) GetBySlug(_ context.Context, slug string) (*Post, error) {
	for _, post := range m.posts {
		if post.Slug == slug {
			return post, nil
		}
	}
	return nil, ErrNotFound
}

func (m *mockRepository) List(_ context.Context, filter PostFilter) ([]*Post, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var posts []*Post
	for _, post := range m.posts {
		if filter.TenantID != uuid.Nil && post.TenantID != filter.TenantID {
			continue
		}
		if !mockMatchesFilter(post, filter) {
			continue
		}
		posts = append(posts, post)
	}
	return posts, nil
}

func (m *mockRepository) Create(_ context.Context, post *Post) error {
	if _, ok := m.posts[post.ID]; ok {
		return ErrAlreadyExists
	}
	cp := *post
	m.posts[post.ID] = &cp
	return nil
}

func (m *mockRepository) Update(_ context.Context, post *Post) error {
	if _, ok := m.posts[post.ID]; !ok {
		return ErrNotFound
	}
	cp := *post
	m.posts[post.ID] = &cp
	return nil
}

func (m *mockRepository) Delete(_ context.Context, id PostID) error {
	if _, ok := m.posts[id]; !ok {
		return ErrNotFound
	}
	delete(m.posts, id)
	return nil
}

func mockMatchesFilter(post *Post, filter PostFilter) bool {
	if len(filter.Statuses) > 0 {
		found := false
		for _, s := range filter.Statuses {
			if post.Status == s {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
