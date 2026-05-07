package core

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"
)

// WorkerConfig конфигурация Worker'а.
type WorkerConfig struct {
	PollInterval    time.Duration
	BackoffSchedule []time.Duration
	MaxAttempts     int
	Logger          *log.Logger
}

// Worker обрабатывает outbox-очередь.
type Worker struct {
	outbox    OutboxRepository
	posts     PostRepository
	publisher Publisher
	clock     Clock
	cfg       WorkerConfig
}

// NewWorker создаёт worker. cfg-поля с zero-value заменяются на defaults.
func NewWorker(outbox OutboxRepository, posts PostRepository, pub Publisher, clock Clock, cfg WorkerConfig) *Worker {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 10 * time.Second
	}
	if len(cfg.BackoffSchedule) == 0 {
		cfg.BackoffSchedule = DefaultBackoffSchedule
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 5
	}
	if cfg.Logger == nil {
		cfg.Logger = log.Default()
	}
	return &Worker{outbox: outbox, posts: posts, publisher: pub, clock: clock, cfg: cfg}
}

// Run запускает poll-loop до отмены ctx. Возвращает ctx.Err().
func (w *Worker) Run(ctx context.Context) error {
	w.cfg.Logger.Printf("worker: started (poll=%s, max_attempts=%d)", w.cfg.PollInterval, w.cfg.MaxAttempts)
	t := time.NewTicker(w.cfg.PollInterval)
	defer t.Stop()
	for {
		// Drain pending entries в текущем тике.
		for {
			processed, err := w.processOne(ctx)
			if err != nil {
				w.cfg.Logger.Printf("worker: process error: %v", err)
			}
			if !processed {
				break
			}
		}
		select {
		case <-ctx.Done():
			w.cfg.Logger.Printf("worker: stopped (%v)", ctx.Err())
			return ctx.Err()
		case <-t.C:
		}
	}
}

// processOne пытается обработать одну запись. Возвращает (processed, err) —
// processed=false означает, что очередь пуста.
func (w *Worker) processOne(ctx context.Context) (bool, error) {
	now := w.clock.Now()
	entry, err := w.outbox.ClaimNext(ctx, now)
	if err != nil {
		return false, fmt.Errorf("claim: %w", err)
	}
	if entry == nil {
		return false, nil
	}
	w.cfg.Logger.Printf("worker: claimed entry %s for post %s (attempt %d)", entry.ID, entry.PostID, entry.Attempts+1)

	post, err := w.posts.GetByID(ctx, entry.PostID)
	if err != nil {
		// Пост удалён — помечаем failed навсегда (нечего публиковать).
		_ = w.outbox.MarkFailed(ctx, entry.ID, fmt.Sprintf("post not found: %v", err), w.clock.Now())
		return true, nil
	}

	updated, pubErr := w.publisher.Publish(ctx, post)
	if pubErr == nil {
		// Успех — обновляем пост и помечаем done.
		updated.Status = StatusPublished
		nowTs := w.clock.Now()
		updated.PublishedAt = &nowTs
		updated.Revision++
		updated.UpdatedAt = nowTs
		if uErr := w.posts.Update(ctx, updated); uErr != nil {
			w.cfg.Logger.Printf("worker: update post after publish failed: %v", uErr)
			// Помечаем retry, чтобы попробовать снова.
			return w.markRetryOrFail(ctx, entry, fmt.Errorf("post update: %w", uErr)), nil
		}
		if dErr := w.outbox.MarkDone(ctx, entry.ID, w.clock.Now()); dErr != nil {
			w.cfg.Logger.Printf("worker: mark done failed: %v", dErr)
		}
		w.cfg.Logger.Printf("worker: published %s", entry.PostID)
		return true, nil
	}

	w.cfg.Logger.Printf("worker: publish failed for %s: %v", entry.PostID, pubErr)
	w.markRetryOrFail(ctx, entry, pubErr)
	return true, nil
}

// markRetryOrFail обновляет outbox-entry: retry с backoff или permanent fail.
// Возвращает true если запись была действительно обновлена.
func (w *Worker) markRetryOrFail(ctx context.Context, entry *OutboxEntry, cause error) bool {
	attempts := entry.Attempts + 1
	now := w.clock.Now()
	if attempts >= entry.MaxAttempts {
		_ = w.outbox.MarkFailed(ctx, entry.ID, cause.Error(), now)
		// Проставляем post.Status=failed.
		if post, err := w.posts.GetByID(ctx, entry.PostID); err == nil {
			post.Status = StatusFailed
			post.Revision++
			post.UpdatedAt = now
			_ = w.posts.Update(ctx, post)
		}
		w.cfg.Logger.Printf("worker: entry %s permanently failed after %d attempts", entry.ID, attempts)
		return true
	}
	backoff := ComputeBackoff(w.cfg.BackoffSchedule, attempts)
	nextAt := now.Add(backoff)
	if err := w.outbox.MarkRetry(ctx, entry.ID, attempts, nextAt, cause.Error(), now); err != nil {
		w.cfg.Logger.Printf("worker: mark retry failed: %v", err)
		return false
	}
	w.cfg.Logger.Printf("worker: entry %s scheduled for retry in %s (attempt %d/%d)", entry.ID, backoff, attempts, entry.MaxAttempts)
	return true
}

// EnqueueForPublish удобный helper — кладёт пост в очередь на публикацию.
func EnqueueForPublish(ctx context.Context, outbox OutboxRepository, post *Post, scheduledAt time.Time) (*OutboxEntry, error) {
	if post == nil {
		return nil, errors.New("post is nil")
	}
	if scheduledAt.IsZero() {
		scheduledAt = time.Now().UTC()
	}
	entry := &OutboxEntry{
		PostID:        post.ID,
		TenantID:      post.TenantID,
		Kind:          OutboxKindPublish,
		Status:        OutboxStatusPending,
		MaxAttempts:   5,
		NextAttemptAt: scheduledAt,
	}
	if err := outbox.Enqueue(ctx, entry); err != nil {
		return nil, err
	}
	return entry, nil
}
