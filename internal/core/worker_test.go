package core

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// --- mocks ---

type mockOutbox struct {
	mu      sync.Mutex
	entries map[uuid.UUID]*OutboxEntry
	queue   []uuid.UUID
}

func newMockOutbox() *mockOutbox {
	return &mockOutbox{entries: map[uuid.UUID]*OutboxEntry{}}
}

func (m *mockOutbox) Enqueue(_ context.Context, e *OutboxEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	m.entries[e.ID] = e
	m.queue = append(m.queue, e.ID)
	return nil
}

func (m *mockOutbox) ClaimNext(_ context.Context, now time.Time) (*OutboxEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, id := range m.queue {
		e := m.entries[id]
		if e.Status == OutboxStatusPending && !e.NextAttemptAt.After(now) {
			e.Status = OutboxStatusInFlight
			m.queue = append(m.queue[:i], m.queue[i+1:]...)
			return e, nil
		}
	}
	return nil, nil //nolint:nilnil // sentinel "очередь пуста"
}

func (m *mockOutbox) MarkDone(_ context.Context, id uuid.UUID, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entries[id]
	if !ok {
		return ErrNotFound
	}
	e.Status = OutboxStatusDone
	e.UpdatedAt = now
	return nil
}

func (m *mockOutbox) MarkRetry(_ context.Context, id uuid.UUID, attempts int, nextAt time.Time, errMsg string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entries[id]
	if !ok {
		return ErrNotFound
	}
	e.Status = OutboxStatusPending
	e.Attempts = attempts
	e.NextAttemptAt = nextAt
	e.LastError = errMsg
	e.UpdatedAt = now
	m.queue = append(m.queue, id)
	return nil
}

func (m *mockOutbox) MarkFailed(_ context.Context, id uuid.UUID, errMsg string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entries[id]
	if !ok {
		return ErrNotFound
	}
	e.Status = OutboxStatusFailed
	e.LastError = errMsg
	e.UpdatedAt = now
	return nil
}

func (m *mockOutbox) List(_ context.Context, _ OutboxFilter) ([]*OutboxEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*OutboxEntry, 0, len(m.entries))
	for _, e := range m.entries {
		out = append(out, e)
	}
	return out, nil
}

func (m *mockOutbox) SweepStuck(_ context.Context, threshold time.Duration, now time.Time) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cutoff := now.Add(-threshold)
	count := 0
	for _, e := range m.entries {
		if e.Status == OutboxStatusInFlight && e.UpdatedAt.Before(cutoff) {
			e.Status = OutboxStatusPending
			e.UpdatedAt = now
			m.queue = append(m.queue, e.ID)
			count++
		}
	}
	return count, nil
}

func (m *mockOutbox) GetByID(_ context.Context, id uuid.UUID) (*OutboxEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entries[id]
	if !ok {
		return nil, ErrNotFound
	}
	return e, nil
}

type mockPosts struct {
	mu    sync.Mutex
	posts map[PostID]*Post
}

func newMockPosts() *mockPosts { return &mockPosts{posts: map[PostID]*Post{}} }

func (m *mockPosts) GetByID(_ context.Context, id PostID) (*Post, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.posts[id]
	if !ok {
		return nil, ErrNotFound
	}
	return p, nil
}
func (m *mockPosts) Update(_ context.Context, p *Post) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.posts[p.ID] = p
	return nil
}

// minimal stubs to satisfy PostRepository interface.
func (m *mockPosts) Create(_ context.Context, p *Post) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.posts[p.ID] = p
	return nil
}
func (m *mockPosts) Delete(_ context.Context, id PostID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.posts, id)
	return nil
}
func (m *mockPosts) GetBySlug(_ context.Context, _ string) (*Post, error) { return nil, ErrNotFound }
func (m *mockPosts) List(_ context.Context, _ PostFilter) ([]*Post, error) {
	return nil, nil
}

type mockPublisher struct {
	mu    sync.Mutex
	calls int
	fail  int // первые fail вызовов возвращают error
	err   error
}

func (p *mockPublisher) Publish(_ context.Context, post *Post) (*Post, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	if p.calls <= p.fail {
		return nil, p.err
	}
	return post, nil
}

type fixedClock struct{ t time.Time }

func (c *fixedClock) Now() time.Time { return c.t }

// --- tests ---

func TestComputeBackoff(t *testing.T) {
	sched := []time.Duration{1 * time.Minute, 5 * time.Minute, 25 * time.Minute}
	if got := ComputeBackoff(sched, 1); got != 1*time.Minute {
		t.Errorf("attempt 1: got %v", got)
	}
	if got := ComputeBackoff(sched, 3); got != 25*time.Minute {
		t.Errorf("attempt 3: got %v", got)
	}
	if got := ComputeBackoff(sched, 100); got != 25*time.Minute {
		t.Errorf("overflow: got %v (want last)", got)
	}
	if got := ComputeBackoff(nil, 1); got != 1*time.Minute {
		t.Errorf("nil schedule: got %v", got)
	}
}

func TestWorker_ProcessOne_Success(t *testing.T) {
	now := time.Now().UTC()
	clock := &fixedClock{t: now}
	outbox := newMockOutbox()
	posts := newMockPosts()
	postID := PostID(uuid.New())
	posts.posts[postID] = &Post{ID: postID, Status: StatusReady, Content: "x", Title: "T"}
	_ = outbox.Enqueue(context.Background(), &OutboxEntry{
		PostID: postID, Status: OutboxStatusPending, MaxAttempts: 3, NextAttemptAt: now,
	})

	pub := &mockPublisher{}
	w := NewWorker(outbox, posts, pub, clock, WorkerConfig{PollInterval: time.Second, MaxAttempts: 3})

	processed, err := w.processOne(context.Background())
	if err != nil || !processed {
		t.Fatalf("expected processed, got %v %v", processed, err)
	}
	if posts.posts[postID].Status != StatusPublished {
		t.Errorf("expected post status published, got %s", posts.posts[postID].Status)
	}
	for _, e := range outbox.entries {
		if e.Status != OutboxStatusDone {
			t.Errorf("expected entry done, got %s", e.Status)
		}
	}
}

func TestWorker_ProcessOne_RetryThenSuccess(t *testing.T) {
	now := time.Now().UTC()
	clock := &fixedClock{t: now}
	outbox := newMockOutbox()
	posts := newMockPosts()
	postID := PostID(uuid.New())
	posts.posts[postID] = &Post{ID: postID, Status: StatusReady, Content: "x", Title: "T"}
	_ = outbox.Enqueue(context.Background(), &OutboxEntry{
		PostID: postID, Status: OutboxStatusPending, MaxAttempts: 3, NextAttemptAt: now,
	})
	pub := &mockPublisher{fail: 1, err: errors.New("transient")}
	w := NewWorker(outbox, posts, pub, clock, WorkerConfig{MaxAttempts: 3})

	// First processOne: fail → retry.
	if processed, _ := w.processOne(context.Background()); !processed {
		t.Fatal("expected processed")
	}
	var e *OutboxEntry
	for _, v := range outbox.entries {
		e = v
	}
	if e.Status != OutboxStatusPending {
		t.Errorf("expected pending after retry, got %s", e.Status)
	}
	if e.Attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", e.Attempts)
	}
	// Advance clock past backoff.
	clock.t = e.NextAttemptAt.Add(time.Second)
	// Second processOne: success.
	if processed, _ := w.processOne(context.Background()); !processed {
		t.Fatal("expected processed second time")
	}
	if e.Status != OutboxStatusDone {
		t.Errorf("expected done, got %s", e.Status)
	}
}

func TestWorker_ProcessOne_PermanentFail(t *testing.T) {
	now := time.Now().UTC()
	clock := &fixedClock{t: now}
	outbox := newMockOutbox()
	posts := newMockPosts()
	postID := PostID(uuid.New())
	posts.posts[postID] = &Post{ID: postID, Status: StatusReady, Content: "x"}
	_ = outbox.Enqueue(context.Background(), &OutboxEntry{
		PostID: postID, Status: OutboxStatusPending, MaxAttempts: 1, NextAttemptAt: now,
	})
	pub := &mockPublisher{fail: 10, err: errors.New("permanent")}
	w := NewWorker(outbox, posts, pub, clock, WorkerConfig{MaxAttempts: 1})

	if processed, _ := w.processOne(context.Background()); !processed {
		t.Fatal("expected processed")
	}
	var e *OutboxEntry
	for _, v := range outbox.entries {
		e = v
	}
	if e.Status != OutboxStatusFailed {
		t.Errorf("expected failed, got %s", e.Status)
	}
	if posts.posts[postID].Status != StatusFailed {
		t.Errorf("expected post failed, got %s", posts.posts[postID].Status)
	}
}

func TestWorker_ProcessOne_EmptyQueue(t *testing.T) {
	w := NewWorker(newMockOutbox(), newMockPosts(), &mockPublisher{}, &fixedClock{t: time.Now()}, WorkerConfig{})
	processed, err := w.processOne(context.Background())
	if err != nil || processed {
		t.Errorf("expected not processed, got %v %v", processed, err)
	}
}

func TestWorker_SweepStuckOnRun(t *testing.T) {
	now := time.Now().UTC()
	clock := &fixedClock{t: now}
	outbox := newMockOutbox()
	postID := PostID(uuid.New())

	// Имитируем stuck-запись: insert + ручной in_flight + старый updated_at.
	stuck := &OutboxEntry{
		ID:            uuid.New(),
		PostID:        postID,
		Status:        OutboxStatusInFlight,
		MaxAttempts:   3,
		NextAttemptAt: now.Add(-time.Hour),
		UpdatedAt:     now.Add(-30 * time.Minute),
	}
	outbox.entries[stuck.ID] = stuck

	posts := newMockPosts()
	posts.posts[postID] = &Post{ID: postID, Status: StatusReady, Content: "x"}
	pub := &mockPublisher{}
	w := NewWorker(outbox, posts, pub, clock, WorkerConfig{StuckThreshold: 10 * time.Minute})

	// processOne НЕ должна найти stuck (он in_flight). После явного sweep — найдёт.
	if processed, _ := w.processOne(context.Background()); processed {
		t.Errorf("expected nothing to process before sweep")
	}
	w.sweepStuck(context.Background())
	if stuck.Status != OutboxStatusPending {
		t.Fatalf("expected pending after sweep, got %s", stuck.Status)
	}
	if processed, _ := w.processOne(context.Background()); !processed {
		t.Errorf("expected processed after sweep")
	}
}

func TestEnqueueForPublish(t *testing.T) {
	outbox := newMockOutbox()
	post := &Post{ID: PostID(uuid.New()), TenantID: uuid.New()}
	entry, err := EnqueueForPublish(context.Background(), outbox, post, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if entry.Kind != OutboxKindPublish || entry.Status != OutboxStatusPending {
		t.Errorf("wrong defaults: %+v", entry)
	}
}

func TestWorker_PublishesPostPublishedEvent(t *testing.T) {
	now := time.Now().UTC()
	clock := &fixedClock{t: now}
	outbox := newMockOutbox()
	posts := newMockPosts()
	postID := PostID(uuid.New())
	posts.posts[postID] = &Post{ID: postID, Status: StatusReady, Title: "T"}
	_ = outbox.Enqueue(context.Background(), &OutboxEntry{
		PostID: postID, Status: OutboxStatusPending, MaxAttempts: 3, NextAttemptAt: now,
	})
	bus := NewMemoryBus(4)
	ch, cancel := bus.Subscribe()
	defer cancel()
	w := NewWorker(outbox, posts, &mockPublisher{}, clock, WorkerConfig{MaxAttempts: 3, Bus: bus})
	if processed, _ := w.processOne(context.Background()); !processed {
		t.Fatal("expected processed")
	}
	select {
	case e := <-ch:
		if e.Topic != "post.published" {
			t.Errorf("topic=%q, want post.published", e.Topic)
		}
		if got, _ := e.Data["id"].(string); got != postID.String() {
			t.Errorf("data.id=%q, want %s", got, postID)
		}
	case <-time.After(time.Second):
		t.Fatal("post.published event not delivered")
	}
}

func TestWorker_PublishesPostFailedEvent(t *testing.T) {
	now := time.Now().UTC()
	clock := &fixedClock{t: now}
	outbox := newMockOutbox()
	posts := newMockPosts()
	postID := PostID(uuid.New())
	posts.posts[postID] = &Post{ID: postID, Status: StatusReady}
	_ = outbox.Enqueue(context.Background(), &OutboxEntry{
		PostID: postID, Status: OutboxStatusPending, MaxAttempts: 1, NextAttemptAt: now,
	})
	bus := NewMemoryBus(4)
	ch, cancel := bus.Subscribe()
	defer cancel()
	w := NewWorker(outbox, posts, &mockPublisher{fail: 10, err: errors.New("perm")}, clock,
		WorkerConfig{MaxAttempts: 1, Bus: bus})
	if processed, _ := w.processOne(context.Background()); !processed {
		t.Fatal("expected processed")
	}
	select {
	case e := <-ch:
		if e.Topic != "post.failed" {
			t.Errorf("topic=%q, want post.failed", e.Topic)
		}
	case <-time.After(time.Second):
		t.Fatal("post.failed event not delivered")
	}
}

func TestWorker_NilBus_DoesNotPanic(t *testing.T) {
	now := time.Now().UTC()
	clock := &fixedClock{t: now}
	outbox := newMockOutbox()
	posts := newMockPosts()
	postID := PostID(uuid.New())
	posts.posts[postID] = &Post{ID: postID, Status: StatusReady}
	_ = outbox.Enqueue(context.Background(), &OutboxEntry{
		PostID: postID, Status: OutboxStatusPending, MaxAttempts: 3, NextAttemptAt: now,
	})
	w := NewWorker(outbox, posts, &mockPublisher{}, clock, WorkerConfig{MaxAttempts: 3})
	if processed, _ := w.processOne(context.Background()); !processed {
		t.Fatal("expected processed")
	}
}
