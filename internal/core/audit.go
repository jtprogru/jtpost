package core

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// AuditAction — категория аудит-события.
type AuditAction string

// Известные audit actions. Список расширяется по мере добавления хуков.
const (
	AuditAuthLoginSuccess AuditAction = "auth.login.success"
	AuditAuthLoginFail    AuditAction = "auth.login.fail"
	AuditAuthLogout       AuditAction = "auth.logout"
	AuditAuthOAuthLogin   AuditAction = "auth.oauth.login"
	AuditTokenIssued      AuditAction = "token.issued"
	AuditTokenRevoked     AuditAction = "token.revoked"
	AuditPostCreated      AuditAction = "post.created"
	AuditPostUpdated      AuditAction = "post.updated"
	AuditPostDeleted      AuditAction = "post.deleted"
	AuditPostPublished    AuditAction = "post.published"
)

// AuditOutcome — итог события (success | failure).
type AuditOutcome string

const (
	// AuditOutcomeSuccess успешное событие.
	AuditOutcomeSuccess AuditOutcome = "success"
	// AuditOutcomeFailure неуспешное событие.
	AuditOutcomeFailure AuditOutcome = "failure"
)

// AuditActorType — категория субъекта (user | token | system | anonymous).
type AuditActorType string

const (
	// AuditActorUser аутентифицированный пользователь.
	AuditActorUser AuditActorType = "user"
	// AuditActorToken запрос с PAT.
	AuditActorToken AuditActorType = "token"
	// AuditActorSystem внутренние/системные события.
	AuditActorSystem AuditActorType = "system"
	// AuditActorAnonymous неаутентифицированный субъект.
	AuditActorAnonymous AuditActorType = "anonymous"
)

// AuditEntry — запись audit log. Append-only.
type AuditEntry struct {
	ID           uuid.UUID
	OccurredAt   time.Time
	TenantID     uuid.UUID // uuid.Nil — system-wide событие
	ActorID      uuid.UUID // uuid.Nil — анонимный/системный субъект
	ActorType    AuditActorType
	Action       AuditAction
	ResourceType string
	ResourceID   string
	Outcome      AuditOutcome
	IP           string
	UserAgent    string
	Metadata     map[string]any // сериализуется в JSON; nil → пустой объект
}

// AuditFilter — фильтры для AuditRepository.List.
type AuditFilter struct {
	TenantID uuid.UUID // uuid.Nil — без фильтрации
	ActorID  uuid.UUID // uuid.Nil — без фильтрации
	Action   AuditAction
	Limit    int
}

// AuditRepository — append-only журнал событий.
type AuditRepository interface {
	Append(ctx context.Context, e *AuditEntry) error
	List(ctx context.Context, filter AuditFilter) ([]*AuditEntry, error)
}

// auditCtxKey — приватный ключ для request-context метаданных (IP/UA).
type auditCtxKey struct{}

// AuditContext — request-scope метаданные для аудита (IP, User-Agent).
type AuditContext struct {
	IP        string
	UserAgent string
}

// WithAuditContext кладёт AuditContext в ctx.
func WithAuditContext(ctx context.Context, ac AuditContext) context.Context {
	return context.WithValue(ctx, auditCtxKey{}, ac)
}

// AuditContextFromContext извлекает AuditContext (или zero-value, false).
func AuditContextFromContext(ctx context.Context) (AuditContext, bool) {
	v, ok := ctx.Value(auditCtxKey{}).(AuditContext)
	return v, ok
}
