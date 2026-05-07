package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/core"
)

type outboxView struct {
	ID            uuid.UUID `json:"id"`
	PostID        uuid.UUID `json:"post_id"`
	TenantID      uuid.UUID `json:"tenant_id"`
	Kind          string    `json:"kind"`
	Status        string    `json:"status"`
	Attempts      int       `json:"attempts"`
	MaxAttempts   int       `json:"max_attempts"`
	NextAttemptAt time.Time `json:"next_attempt_at"`
	LastError     string    `json:"last_error,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func toOutboxView(e *core.OutboxEntry) outboxView {
	return outboxView{
		ID: e.ID, PostID: uuid.UUID(e.PostID), TenantID: e.TenantID,
		Kind: string(e.Kind), Status: string(e.Status),
		Attempts: e.Attempts, MaxAttempts: e.MaxAttempts,
		NextAttemptAt: e.NextAttemptAt, LastError: e.LastError,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
}

// handleOutbox обрабатывает GET /api/outbox?status=pending&limit=50.
func (s *Server) handleOutbox(w http.ResponseWriter, r *http.Request) {
	if s.outbox == nil {
		http.Error(w, "outbox not configured", http.StatusServiceUnavailable)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	filter := core.OutboxFilter{}
	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = core.OutboxStatus(status)
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			filter.Limit = n
		}
	}
	entries, err := s.outbox.List(r.Context(), filter)
	if err != nil {
		s.log.Error("outbox list: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	views := make([]outboxView, 0, len(entries))
	for _, e := range entries {
		views = append(views, toOutboxView(e))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(views)
}

// handleOutboxByID обрабатывает /api/outbox/{id} и /api/outbox/{id}/retry.
func (s *Server) handleOutboxByID(w http.ResponseWriter, r *http.Request) {
	if s.outbox == nil {
		http.Error(w, "outbox not configured", http.StatusServiceUnavailable)
		return
	}
	path := r.URL.Path
	for _, p := range []string{"/api/v1/outbox/", "/api/outbox/"} {
		if rest, ok := strings.CutPrefix(path, p); ok {
			path = rest
			break
		}
	}
	// Retry suffix.
	if rest, ok := strings.CutSuffix(path, "/retry"); ok {
		id, err := uuid.Parse(rest)
		if err != nil {
			http.Error(w, "invalid outbox id", http.StatusBadRequest)
			return
		}
		s.retryOutbox(w, r, id)
		return
	}
	id, err := uuid.Parse(path)
	if err != nil {
		http.Error(w, "invalid outbox id", http.StatusBadRequest)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	entry, err := s.outbox.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			http.Error(w, "outbox entry not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toOutboxView(entry))
}

// retryOutbox сбрасывает entry в pending с attempts=0, next_attempt_at=now.
func (s *Server) retryOutbox(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	now := time.Now().UTC()
	entry, err := s.outbox.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			http.Error(w, "outbox entry not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if entry.Status == core.OutboxStatusDone {
		http.Error(w, "entry already done — nothing to retry", http.StatusConflict)
		return
	}
	if err := s.outbox.MarkRetry(r.Context(), id, 0, now, "manual retry", now); err != nil {
		s.log.Error("retry outbox: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	updated, _ := s.outbox.GetByID(r.Context(), id)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	if updated != nil {
		_ = json.NewEncoder(w).Encode(toOutboxView(updated))
	}
}
