// Package postgres реализует core.PostRepository поверх pgxpool.Pool.
package postgres

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jtprogru/jtpost/internal/core"
)

// toPgUUID конвертирует uuid.UUID в pgtype.UUID.
func toPgUUID(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: u, Valid: true}
}

// fromPgUUID конвертирует pgtype.UUID в uuid.UUID.
func fromPgUUID(p pgtype.UUID) uuid.UUID {
	if !p.Valid {
		return uuid.Nil
	}
	return uuid.UUID(p.Bytes)
}

// toPgTimestamp конвертирует *time.Time в pgtype.Timestamptz.
func toPgTimestamp(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{Valid: false}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// toPgTimestampVal конвертирует обязательное time.Time в pgtype.Timestamptz.
func toPgTimestampVal(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// fromPgTimestamp конвертирует pgtype.Timestamptz в *time.Time (nil если invalid).
func fromPgTimestamp(p pgtype.Timestamptz) *time.Time {
	if !p.Valid {
		return nil
	}
	t := p.Time
	return &t
}

// toPgText конвертирует *string в pgtype.Text.
func toPgText(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// toPgTextStr конвертирует обязательную строку в pgtype.Text (используется для telegram_url=ExternalLinks.TelegramURL).
func toPgTextStr(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}

// fromPgText конвертирует pgtype.Text в *string.
func fromPgText(p pgtype.Text) *string {
	if !p.Valid {
		return nil
	}
	s := p.String
	return &s
}

// fromPgTextStr возвращает строку из pgtype.Text ("" если invalid).
func fromPgTextStr(p pgtype.Text) string {
	if !p.Valid {
		return ""
	}
	return p.String
}

// marshalJSON сериализует значение в []byte; nil-slice превращается в "[]"; nil-pointer → SQL NULL ([]byte(nil)).
func marshalJSON(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

// marshalJSONArray сериализует слайс — если nil/пустой, возвращает "[]".
func marshalJSONArray(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if len(b) == 0 || string(b) == "null" {
		return []byte("[]"), nil
	}
	return b, nil
}

// unmarshalJSON десериализует []byte в dest. На ошибке оборачивает в ErrValidation.
// Пустой/nil src и "null" src — допустимый no-op.
func unmarshalJSON(src []byte, dest any) error {
	if len(src) == 0 {
		return nil
	}
	if string(src) == "null" {
		return nil
	}
	if err := json.Unmarshal(src, dest); err != nil {
		return errors.Join(core.ErrValidation, err)
	}
	return nil
}
