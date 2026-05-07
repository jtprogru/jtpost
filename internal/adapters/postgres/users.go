package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jtprogru/jtpost/internal/adapters/postgres/pgdb"
	"github.com/jtprogru/jtpost/internal/core"
)

// UserRepository реализует core.UserRepository поверх pgxpool.Pool.
//
// Это лёгкий "view" поверх того же пула / *pgdb.Queries что и PostRepository —
// multi-interface на одной структуре невозможен (collision с GetByID(PostID)).
type UserRepository struct {
	pool    *pgxpool.Pool
	queries *pgdb.Queries
}

// Users возвращает UserRepository поверх того же пула.
func (r *PostRepository) Users() *UserRepository {
	return &UserRepository{pool: r.pool, queries: r.queries}
}

var _ core.UserRepository = (*UserRepository)(nil)

// Create вставляет нового пользователя.
func (r *UserRepository) Create(ctx context.Context, user *core.User) error {
	if user == nil {
		return fmt.Errorf("%w: user is nil", core.ErrValidation)
	}
	createdAt := user.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	updatedAt := user.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}
	err := r.queries.CreateUser(ctx, pgdb.CreateUserParams{
		ID:           toPgUUID(user.ID),
		TenantID:     toPgUUID(user.TenantID),
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		Role:         string(user.Role),
		CreatedAt:    toPgTimestampVal(createdAt),
		UpdatedAt:    toPgTimestampVal(updatedAt),
	})
	if err != nil {
		if isPgUniqueViolation(err) {
			return core.ErrAlreadyExists
		}
		return err
	}
	return nil
}

// GetByID возвращает пользователя по ID.
func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*core.User, error) {
	row, err := r.queries.GetUserByID(ctx, toPgUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}
	return userFromPgRow(row)
}

// GetByEmail возвращает пользователя по email в рамках tenant.
func (r *UserRepository) GetByEmail(ctx context.Context, tenantID uuid.UUID, email string) (*core.User, error) {
	row, err := r.queries.GetUserByEmail(ctx, pgdb.GetUserByEmailParams{
		TenantID: toPgUUID(tenantID),
		Email:    email,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}
	return userFromPgRow(row)
}

// Update обновляет email/password_hash/role/updated_at.
func (r *UserRepository) Update(ctx context.Context, user *core.User) error {
	if user == nil {
		return fmt.Errorf("%w: user is nil", core.ErrValidation)
	}
	updatedAt := user.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	rows, err := r.queries.UpdateUser(ctx, pgdb.UpdateUserParams{
		ID:           toPgUUID(user.ID),
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		Role:         string(user.Role),
		UpdatedAt:    toPgTimestampVal(updatedAt),
	})
	if err != nil {
		if isPgUniqueViolation(err) {
			return core.ErrAlreadyExists
		}
		return err
	}
	if rows == 0 {
		return core.ErrNotFound
	}
	return nil
}

// Delete удаляет пользователя; FK ON DELETE CASCADE удалит токены.
func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	rows, err := r.queries.DeleteUser(ctx, toPgUUID(id))
	if err != nil {
		return err
	}
	if rows == 0 {
		return core.ErrNotFound
	}
	return nil
}

// List возвращает пользователей tenant'а.
func (r *UserRepository) List(ctx context.Context, tenantID uuid.UUID) ([]*core.User, error) {
	rows, err := r.queries.ListUsersByTenant(ctx, toPgUUID(tenantID))
	if err != nil {
		return nil, err
	}
	out := make([]*core.User, 0, len(rows))
	for _, row := range rows {
		u, err := userFromPgRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, nil
}

// Count возвращает количество пользователей tenant'а.
func (r *UserRepository) Count(ctx context.Context, tenantID uuid.UUID) (int64, error) {
	return r.queries.CountUsersByTenant(ctx, toPgUUID(tenantID))
}

// CountOwners возвращает количество owner-пользователей tenant'а.
func (r *UserRepository) CountOwners(ctx context.Context, tenantID uuid.UUID) (int64, error) {
	return r.queries.CountOwnersByTenant(ctx, toPgUUID(tenantID))
}

// userFromPgRow конвертирует pgdb.User в *core.User.
func userFromPgRow(row pgdb.User) (*core.User, error) {
	return &core.User{
		ID:           fromPgUUID(row.ID),
		TenantID:     fromPgUUID(row.TenantID),
		Email:        row.Email,
		PasswordHash: row.PasswordHash,
		Role:         core.Role(row.Role),
		CreatedAt:    row.CreatedAt.Time,
		UpdatedAt:    row.UpdatedAt.Time,
	}, nil
}

// isPgUniqueViolation возвращает true если ошибка — Postgres unique_violation (23505).
func isPgUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
