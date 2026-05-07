package core

import (
	"errors"
	"testing"
)

func TestErrorsExist(t *testing.T) {
	tt := []struct {
		name string
		err  error
	}{
		{"ErrConflict", ErrConflict},
		{"ErrMigrationFailed", ErrMigrationFailed},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err == nil {
				t.Fatalf("%s should not be nil", tc.name)
			}
			if !errors.Is(tc.err, tc.err) {
				t.Fatalf("%s: errors.Is self-match must hold", tc.name)
			}
		})
	}
}

func TestErrorsAreDistinct(t *testing.T) {
	tt := []struct {
		name string
		a, b error
	}{
		{"Conflict_vs_NotFound", ErrConflict, ErrNotFound},
		{"Conflict_vs_TenantMismatch", ErrConflict, ErrTenantMismatch},
		{"Conflict_vs_Validation", ErrConflict, ErrValidation},
		{"MigrationFailed_vs_ConfigInvalid", ErrMigrationFailed, ErrConfigInvalid},
		{"MigrationFailed_vs_Conflict", ErrMigrationFailed, ErrConflict},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			if errors.Is(tc.a, tc.b) {
				t.Fatalf("%s: errors must be distinct", tc.name)
			}
		})
	}
}
