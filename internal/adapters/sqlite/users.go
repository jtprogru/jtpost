package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jtprogru/jtpost/internal/adapters/sqlite/sqlitedb"
	"github.com/jtprogru/jtpost/internal/core"
)

// UserRepository реализует core.UserRepository поверх SQLite.
//
// Это лёгкий "view" поверх того же *sql.DB / *sqlitedb.Queries что и
// PostRepository — multi-interface на одной структуре невозможен из-за
// конфликта имён методов (GetByID(PostID) vs GetByID(uuid.UUID)). Поэтому
// используем отдельный тип, разделяющий нижележащий пул соединений.
type UserRepository struct {
	db      *sql.DB
	queries *sqlitedb.Queries
}

// Users возвращает UserRepository поверх того же подключения.
func (r *PostRepository) Users() *UserRepository {
	return &UserRepository{db: r.db, queries: r.queries}
}

// Compile-time проверка соответствия интерфейсу.
var _ core.UserRepository = (*UserRepository)(nil)

// Create вставляет нового пользователя. На UNIQUE(tenant_id,email) → ErrAlreadyExists.
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
	err := r.queries.CreateUser(ctx, sqlitedb.CreateUserParams{
		ID:           user.ID.String(),
		TenantID:     user.TenantID.String(),
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		Role:         string(user.Role),
		CreatedAt:    createdAt.UTC().Format(time.RFC3339),
		UpdatedAt:    updatedAt.UTC().Format(time.RFC3339),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return core.ErrAlreadyExists
		}
		return err
	}
	return nil
}

// GetByID возвращает пользователя по ID.
func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*core.User, error) {
	row, err := r.queries.GetUserByID(ctx, id.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}
	return userFromRow(row)
}

// GetByEmail возвращает пользователя по email в рамках tenant.
func (r *UserRepository) GetByEmail(ctx context.Context, tenantID uuid.UUID, email string) (*core.User, error) {
	row, err := r.queries.GetUserByEmail(ctx, sqlitedb.GetUserByEmailParams{
		TenantID: tenantID.String(),
		Email:    email,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}
	return userFromRow(row)
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
	rows, err := r.queries.UpdateUser(ctx, sqlitedb.UpdateUserParams{
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		Role:         string(user.Role),
		UpdatedAt:    updatedAt.UTC().Format(time.RFC3339),
		ID:           user.ID.String(),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return core.ErrAlreadyExists
		}
		return err
	}
	if rows == 0 {
		return core.ErrNotFound
	}
	return nil
}

// Delete удаляет пользователя; FK ON DELETE CASCADE удалит его токены.
func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	rows, err := r.queries.DeleteUser(ctx, id.String())
	if err != nil {
		return err
	}
	if rows == 0 {
		return core.ErrNotFound
	}
	return nil
}

// List возвращает пользователей tenant'а, ORDER BY created_at ASC.
func (r *UserRepository) List(ctx context.Context, tenantID uuid.UUID) ([]*core.User, error) {
	rows, err := r.queries.ListUsersByTenant(ctx, tenantID.String())
	if err != nil {
		return nil, err
	}
	out := make([]*core.User, 0, len(rows))
	for _, row := range rows {
		u, err := userFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, nil
}

// Count возвращает количество пользователей tenant'а.
func (r *UserRepository) Count(ctx context.Context, tenantID uuid.UUID) (int64, error) {
	return r.queries.CountUsersByTenant(ctx, tenantID.String())
}

// CountOwners возвращает количество owner-пользователей tenant'а.
func (r *UserRepository) CountOwners(ctx context.Context, tenantID uuid.UUID) (int64, error) {
	return r.queries.CountOwnersByTenant(ctx, tenantID.String())
}

// userFromRow конвертирует sqlitedb.User в *core.User.
func userFromRow(row sqlitedb.User) (*core.User, error) {
	id, err := uuid.Parse(row.ID)
	if err != nil {
		return nil, errors.Join(core.ErrValidation, fmt.Errorf("invalid user id: %w", err))
	}
	tenantID, err := uuid.Parse(row.TenantID)
	if err != nil {
		return nil, errors.Join(core.ErrValidation, fmt.Errorf("invalid tenant_id: %w", err))
	}
	createdAt, err := time.Parse(time.RFC3339, row.CreatedAt)
	if err != nil {
		return nil, errors.Join(core.ErrValidation, fmt.Errorf("invalid created_at: %w", err))
	}
	updatedAt, err := time.Parse(time.RFC3339, row.UpdatedAt)
	if err != nil {
		return nil, errors.Join(core.ErrValidation, fmt.Errorf("invalid updated_at: %w", err))
	}
	return &core.User{
		ID:           id,
		TenantID:     tenantID,
		Email:        row.Email,
		PasswordHash: row.PasswordHash,
		Role:         core.Role(row.Role),
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}, nil
}

// isUniqueViolation проверяет SQLite ошибку UNIQUE constraint.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "constraint failed: UNIQUE") ||
		strings.Contains(msg, "constraint failed (2067)") ||
		strings.Contains(strings.ToLower(msg), "unique")
}
