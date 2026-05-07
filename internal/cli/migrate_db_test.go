package cli

import "testing"

func TestMaskDSN(t *testing.T) {
	tt := []struct {
		name string
		in   string
		want string
	}{
		{"postgres_with_password", "postgres://user:secret@host/db", "postgres://user:***@host/db"},
		{"postgresql_scheme", "postgresql://u:p@h:5432/d?x=1", "postgresql://u:***@h:5432/d?x=1"},
		{"no_password", "postgres://user@host/db", "postgres://user@host/db"},
		{"no_credentials", "postgres://host/db", "postgres://host/db"},
		{"no_scheme", "host/db", "host/db"},
		{"empty", "", ""},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			if got := maskDSN(tc.in); got != tc.want {
				t.Errorf("maskDSN(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
