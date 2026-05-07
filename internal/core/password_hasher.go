package core

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

// PasswordHasher — абстракция над password hashing. Реализации:
// Argon2idHasher (rw), LegacyBcryptHasher (read-only verify), MultiHasher
// (detection by prefix).
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(hash, password string) error
	NeedsRehash(hash string) bool
}

// Argon2id baseline parameters (OWASP 2024).
const (
	argon2idTime    uint32 = 1
	argon2idMemory  uint32 = 64 * 1024 // 64 MB
	argon2idThreads uint8  = 4
	argon2idKeyLen  uint32 = 32
	argon2idSaltLen        = 16
	argon2idVersion        = argon2.Version

	argon2idPrefix = "$argon2id$"
)

// Argon2idHasher — current default password hasher.
type Argon2idHasher struct{}

// NewArgon2idHasher создаёт hasher с baseline params.
func NewArgon2idHasher() *Argon2idHasher { return &Argon2idHasher{} }

// Hash возвращает строку формата `$argon2id$v=19$m=65536,t=1,p=4$<b64-salt>$<b64-hash>`.
func (h *Argon2idHasher) Hash(password string) (string, error) {
	salt := make([]byte, argon2idSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("argon2id: read salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argon2idTime, argon2idMemory, argon2idThreads, argon2idKeyLen)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2idVersion,
		argon2idMemory, argon2idTime, argon2idThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// Verify парсит формат, перехеширует password с теми же params, сравнивает.
func (h *Argon2idHasher) Verify(hash, password string) error {
	parts := strings.Split(hash, "$")
	// Format: $argon2id$v=...$m=...,t=...,p=...$salt$hash → 6 после Split (+1 пустой первый).
	if len(parts) != 6 {
		return ErrUnauthorized
	}
	if parts[1] != "argon2id" {
		return ErrUnauthorized
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return ErrUnauthorized
	}
	var memory uint32
	var time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return ErrUnauthorized
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return ErrUnauthorized
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return ErrUnauthorized
	}
	got := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(expected))) //nolint:gosec // G115: len(expected) ограничен Argon2 hash длиной (32 байта)
	if subtle.ConstantTimeCompare(got, expected) != 1 {
		return ErrUnauthorized
	}
	return nil
}

// NeedsRehash возвращает false для argon2id-format (current).
func (h *Argon2idHasher) NeedsRehash(hash string) bool {
	return !strings.HasPrefix(hash, argon2idPrefix)
}

// LegacyBcryptHasher — read-only verifier для существующих F4a-hashes.
type LegacyBcryptHasher struct{}

// NewLegacyBcryptHasher создаёт legacy verifier.
func NewLegacyBcryptHasher() *LegacyBcryptHasher { return &LegacyBcryptHasher{} }

// Hash deprecated — bcrypt больше не используется для новых passwords.
func (h *LegacyBcryptHasher) Hash(_ string) (string, error) {
	return "", errors.New("legacy bcrypt hasher: Hash() is deprecated; use Argon2idHasher")
}

// Verify через bcrypt.CompareHashAndPassword.
func (h *LegacyBcryptHasher) Verify(hash, password string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return ErrUnauthorized
	}
	return nil
}

// NeedsRehash всегда true для bcrypt — нужно мигрировать.
func (h *LegacyBcryptHasher) NeedsRehash(_ string) bool { return true }

// MultiHasher делает detection по prefix и делегирует в Argon2id или Bcrypt.
// Hash всегда использует Argon2id.
type MultiHasher struct {
	argon2id *Argon2idHasher
	bcrypt   *LegacyBcryptHasher
}

// NewMultiHasher создаёт hasher с обоими backends.
func NewMultiHasher() *MultiHasher {
	return &MultiHasher{
		argon2id: NewArgon2idHasher(),
		bcrypt:   NewLegacyBcryptHasher(),
	}
}

// Hash всегда argon2id.
func (m *MultiHasher) Hash(password string) (string, error) {
	return m.argon2id.Hash(password)
}

// Verify детектирует по prefix.
func (m *MultiHasher) Verify(hash, password string) error {
	switch {
	case strings.HasPrefix(hash, argon2idPrefix):
		return m.argon2id.Verify(hash, password)
	case strings.HasPrefix(hash, "$2a$"), strings.HasPrefix(hash, "$2b$"), strings.HasPrefix(hash, "$2y$"):
		return m.bcrypt.Verify(hash, password)
	default:
		return ErrUnauthorized
	}
}

// NeedsRehash возвращает true для не-argon2id hashes.
func (m *MultiHasher) NeedsRehash(hash string) bool {
	return !strings.HasPrefix(hash, argon2idPrefix)
}

// HasherFromConfig создаёт PasswordHasher по cfg.Auth.PasswordHasher значению.
// "" / "auto" / "argon2id" / "bcrypt" → MultiHasher (write argon2, read both).
func HasherFromConfig(passwordHasher string) PasswordHasher {
	_ = passwordHasher // в F4c — единый MultiHasher; semantics-разные конфиги резерв на follow-up
	return NewMultiHasher()
}

// _ — keep strconv import if used elsewhere; this file does not use it directly,
// but keeping for future parameter parsing.
var _ = strconv.Itoa
