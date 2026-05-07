package core

import (
	"context"

	"github.com/google/uuid"
)

// ctxKey тип ключа для context-values, ограничивающих scope операций.
type ctxKey int

const (
	tenantKey ctxKey = iota
	authorKey
)

// WithTenant возвращает новый контекст с установленным TenantID.
func WithTenant(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, tenantKey, id)
}

// TenantFromContext извлекает TenantID из контекста. Возвращает (uuid.Nil, false),
// если ключ не задан.
func TenantFromContext(ctx context.Context) (uuid.UUID, bool) {
	if ctx == nil {
		return uuid.Nil, false
	}
	v, ok := ctx.Value(tenantKey).(uuid.UUID)
	if !ok || v == uuid.Nil {
		return uuid.Nil, false
	}
	return v, true
}

// WithAuthor возвращает новый контекст с установленным AuthorID.
func WithAuthor(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, authorKey, id)
}

// AuthorFromContext извлекает AuthorID из контекста.
func AuthorFromContext(ctx context.Context) (uuid.UUID, bool) {
	if ctx == nil {
		return uuid.Nil, false
	}
	v, ok := ctx.Value(authorKey).(uuid.UUID)
	if !ok || v == uuid.Nil {
		return uuid.Nil, false
	}
	return v, true
}
