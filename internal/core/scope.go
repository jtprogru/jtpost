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
	userKey
	roleKey
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

// WithUser кладёт User в контекст (используется BearerTokenMiddleware).
func WithUser(ctx context.Context, u *User) context.Context {
	if u == nil {
		return ctx
	}
	return context.WithValue(ctx, userKey, u)
}

// UserFromContext извлекает User. Возвращает (nil, false) если ключ не задан.
func UserFromContext(ctx context.Context) (*User, bool) {
	if ctx == nil {
		return nil, false
	}
	v, ok := ctx.Value(userKey).(*User)
	if !ok || v == nil {
		return nil, false
	}
	return v, true
}

// WithRole кладёт Role в контекст.
func WithRole(ctx context.Context, r Role) context.Context {
	if r == "" {
		return ctx
	}
	return context.WithValue(ctx, roleKey, r)
}

// RoleFromContext извлекает Role.
func RoleFromContext(ctx context.Context) (Role, bool) {
	if ctx == nil {
		return "", false
	}
	v, ok := ctx.Value(roleKey).(Role)
	if !ok || v == "" {
		return "", false
	}
	return v, true
}
