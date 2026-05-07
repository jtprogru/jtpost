package core

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// AuditService — тонкая обёртка над AuditRepository. Заполняет
// occurred_at/ID если не заданы и подмешивает request-context (IP/UA, actor)
// из ctx. Ошибки append-а не пробрасывает (audit — best-effort, не должен
// ломать основную операцию); вызывающий слой логирует через возвращаемую
// ошибку при желании.
type AuditService struct {
	repo  AuditRepository
	clock Clock
}

// NewAuditService создаёт AuditService.
func NewAuditService(repo AuditRepository, clock Clock) *AuditService {
	if clock == nil {
		clock = SystemClock{}
	}
	return &AuditService{repo: repo, clock: clock}
}

// Log записывает событие. Заполняет недостающие поля из ctx (User → ActorID,
// AuditContext → IP/UA). Возвращает ошибку для логирования вызывающим кодом.
func (s *AuditService) Log(ctx context.Context, e AuditEntry) error {
	if s == nil || s.repo == nil {
		return nil
	}
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = s.clock.Now().UTC()
	} else {
		e.OccurredAt = e.OccurredAt.UTC()
	}
	if e.Outcome == "" {
		e.Outcome = AuditOutcomeSuccess
	}
	if e.ActorID == uuid.Nil {
		if u, ok := UserFromContext(ctx); ok && u != nil {
			e.ActorID = u.ID
			if e.TenantID == uuid.Nil {
				e.TenantID = u.TenantID
			}
		}
	}
	if e.ActorType == "" {
		switch {
		case e.ActorID != uuid.Nil:
			e.ActorType = AuditActorUser
		default:
			e.ActorType = AuditActorAnonymous
		}
	}
	if ac, ok := AuditContextFromContext(ctx); ok {
		if e.IP == "" {
			e.IP = ac.IP
		}
		if e.UserAgent == "" {
			e.UserAgent = ac.UserAgent
		}
	}
	if e.Metadata == nil {
		e.Metadata = map[string]any{}
	}
	// Используем Background для append, чтобы отмена request-ctx не теряла
	// аудит-запись (лучше иметь "медленный" аудит, чем потерянный).
	logCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.repo.Append(logCtx, &e) //nolint:contextcheck // detached: audit append не должен теряться при отмене request ctx
}
