package core

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestArgon2idHasher_RoundTrip(t *testing.T) {
	h := NewArgon2idHasher()
	hash, err := h.Hash("password123")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Verify(hash, "password123"); err != nil {
		t.Errorf("verify same: %v", err)
	}
}

func TestArgon2idHasher_WrongPassword(t *testing.T) {
	h := NewArgon2idHasher()
	hash, _ := h.Hash("password123")
	if err := h.Verify(hash, "wrong"); !errors.Is(err, ErrUnauthorized) {
		t.Errorf("want ErrUnauthorized, got %v", err)
	}
}

func TestArgon2idHasher_FormatString(t *testing.T) {
	h := NewArgon2idHasher()
	hash, _ := h.Hash("p")
	rgx := regexp.MustCompile(`^\$argon2id\$v=19\$m=65536,t=1,p=4\$[A-Za-z0-9+/]+\$[A-Za-z0-9+/]+$`)
	if !rgx.MatchString(hash) {
		t.Errorf("format mismatch: %q", hash)
	}
}

func TestArgon2idHasher_NeedsRehash(t *testing.T) {
	h := NewArgon2idHasher()
	if h.NeedsRehash("$argon2id$v=19$m=65536,t=1,p=4$abc$def") {
		t.Error("argon2id hash должен NOT need rehash")
	}
	if !h.NeedsRehash("$2a$10$xxxxxxxxxxxxxxxxxxxxx") {
		t.Error("bcrypt hash должен need rehash")
	}
}

func TestLegacyBcryptHasher_VerifyExisting(t *testing.T) {
	bcryptHash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	h := NewLegacyBcryptHasher()
	if err := h.Verify(string(bcryptHash), "password123"); err != nil {
		t.Errorf("verify ok: %v", err)
	}
	if err := h.Verify(string(bcryptHash), "wrong"); !errors.Is(err, ErrUnauthorized) {
		t.Errorf("verify wrong: want ErrUnauthorized, got %v", err)
	}
}

func TestLegacyBcryptHasher_Hash_Deprecated(t *testing.T) {
	h := NewLegacyBcryptHasher()
	if _, err := h.Hash("p"); err == nil {
		t.Error("Hash() must be deprecated")
	}
}

func TestMultiHasher_DetectArgon2id(t *testing.T) {
	m := NewMultiHasher()
	hash, _ := m.Hash("password123")
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Errorf("Hash должен argon2id, got %q", hash)
	}
	if err := m.Verify(hash, "password123"); err != nil {
		t.Errorf("verify ok: %v", err)
	}
}

func TestMultiHasher_DetectBcrypt(t *testing.T) {
	bcryptHash, _ := bcrypt.GenerateFromPassword([]byte("p"), bcrypt.MinCost)
	m := NewMultiHasher()
	if err := m.Verify(string(bcryptHash), "p"); err != nil {
		t.Errorf("bcrypt verify: %v", err)
	}
}

func TestMultiHasher_UnknownFormat(t *testing.T) {
	m := NewMultiHasher()
	if err := m.Verify("plain-password", "p"); !errors.Is(err, ErrUnauthorized) {
		t.Errorf("unknown format: want ErrUnauthorized, got %v", err)
	}
}

func TestMultiHasher_NeedsRehash(t *testing.T) {
	m := NewMultiHasher()
	tt := []struct {
		hash string
		want bool
	}{
		{"$argon2id$v=19$m=65536,t=1,p=4$abc$def", false},
		{"$2a$10$xxx", true},
		{"$2b$10$xxx", true},
		{"$2y$10$xxx", true},
		{"unknown", true},
	}
	for _, tc := range tt {
		t.Run(tc.hash, func(t *testing.T) {
			if got := m.NeedsRehash(tc.hash); got != tc.want {
				t.Errorf("NeedsRehash(%q)=%v, want %v", tc.hash, got, tc.want)
			}
		})
	}
}
