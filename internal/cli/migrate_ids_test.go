package cli

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/core"
)

func TestHasOldIDFormat(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		expected bool
	}{
		{
			name:     "старый формат с timestamp",
			id:       "1710345678-my-post-slug",
			expected: true,
		},
		{
			name:     "старый формат с коротким slug",
			id:       "1234567890-post",
			expected: true,
		},
		{
			name:     "UUID v7 формат",
			id:       "0195e8d4-3c7a-7b2e-8f3a-9c5d6e4f2a1b",
			expected: false,
		},
		{
			name:     "UUID v4 формат",
			id:       "550e8400-e29b-41d4-a716-446655440000",
			expected: false,
		},
		{
			name:     "некорректный формат",
			id:       "not-a-valid-id",
			expected: false,
		},
		{
			name:     "только timestamp",
			id:       "1710345678",
			expected: false,
		},
		{
			name:     "пустой ID",
			id:       "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasOldIDFormat(tt.id)
			if result != tt.expected {
				t.Errorf("hasOldIDFormat(%q) = %v, expected %v", tt.id, result, tt.expected)
			}
		})
	}
}

func TestConvertOldIDToUUIDv7(t *testing.T) {
	tests := []struct {
		name            string
		oldID           string
		expectValid     bool
		expectUUIDv7    bool
		expectTimestamp int64 // ожидаемый timestamp (с погрешностью)
	}{
		{
			name:            "конвертация старого ID",
			oldID:           "1710345678-my-post-slug",
			expectValid:     true,
			expectUUIDv7:    true,
			expectTimestamp: 1710345678,
		},
		{
			name:            "конвертация с другим timestamp",
			oldID:           "1609459200-another-post",
			expectValid:     true,
			expectUUIDv7:    true,
			expectTimestamp: 1609459200,
		},
		{
			name:            "некорректный старый ID (генерируется новый)",
			oldID:           "invalid-timestamp-post",
			expectValid:     true,
			expectUUIDv7:    false, // Будет UUID v4 или v7 с текущим временем
			expectTimestamp: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertOldIDToUUIDv7(tt.oldID)

			// Проверяем, что результат не пустой
			if result == (core.PostID{}) {
				t.Fatal("получен пустой ID")
			}

			resultStr := result.String()
			t.Logf("конвертация %q → %q", tt.oldID, resultStr)

			// Проверяем, что это валидный UUID
			parsed, err := uuid.Parse(resultStr)
			if err != nil {
				t.Fatalf("неверный UUID формат: %v", err)
			}

			if tt.expectUUIDv7 {
				// Проверяем версию UUID (должна быть 7)
				version := parsed.Version()
				if version != 7 {
					t.Errorf("ожидалась версия UUID 7, получена %d", version)
				}

				// Проверяем, что timestamp в UUID соответствует исходному
				sec, _ := parsed.Time().UnixTime()
				timeDiff := sec - tt.expectTimestamp
				if timeDiff < 0 {
					timeDiff = -timeDiff
				}
				// Допускаем погрешность в 1 секунду из-за округления до миллисекунд
				if timeDiff > 1 {
					t.Errorf("timestamp UUID %d не совпадает с ожидаемым %d (разница: %d)",
						sec, tt.expectTimestamp, timeDiff)
				}
			}
		})
	}
}

func TestGenerateUUIDv7FromTime(t *testing.T) {
	tests := []struct {
		name    string
		timeStr string
		format  string
	}{
		{
			name:    "конкретное время",
			timeStr: "2024-03-13T12:00:00Z",
			format:  time.RFC3339,
		},
		{
			name:    "другое время",
			timeStr: "2021-01-01T00:00:00Z",
			format:  time.RFC3339,
		},
		{
			name:    "текущее время",
			timeStr: time.Now().Format(time.RFC3339),
			format:  time.RFC3339,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testTime, err := time.Parse(tt.format, tt.timeStr)
			if err != nil {
				t.Fatalf("ошибка парсинга времени: %v", err)
			}

			u := generateUUIDv7FromTime(testTime)

			// Проверяем, что UUID не пустой
			if u == uuid.Nil {
				t.Fatal("получен пустой UUID")
			}

			// Проверяем версию
			if u.Version() != 7 {
				t.Errorf("ожидалась версия UUID 7, получена %d", u.Version())
			}

			// Проверяем, что timestamp в UUID близок к исходному
			uuidTime := u.Time()
			expectedMs := testTime.UnixMilli()
			// uuid.Time — это 100-наносекундные интервалы с 1582 года
			// Конвертируем в миллисекунды
			sec, nsec := uuidTime.UnixTime()
			actualMs := sec*1000 + nsec/1_000_000

			diff := actualMs - expectedMs
			if diff < 0 {
				diff = -diff
			}

			// UUID v7 хранит timestamp с точностью до миллисекунд
			// Допускаем небольшую погрешность
			if diff > 1000 { // 1 секунда
				t.Errorf("timestamp UUID расходится с исходным более чем на 1 секунду: %d мс", diff)
			}

			t.Logf("время: %s → UUID: %s → время: %s",
				testTime.Format(time.RFC3339),
				u.String(),
				time.Unix(sec, nsec).Format(time.RFC3339))
		})
	}
}

func TestOldIDPattern(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		expected bool
	}{
		{"валидный старый ID", "1710345678-my-post-slug", true},
		{"короткий slug", "1234567890-post", true},
		{"длинный slug", "1234567890-very-long-slug-with-many-words", true},
		{"с цифрами в slug", "1234567890-post-123", true},
		{"без дефиса после timestamp", "1710345678", false},
		{"не число в начале", "abc-my-post", false},
		{"пустая строка", "", false},
		{"UUID v7", "0195e8d4-3c7a-7b2e-8f3a-9c5d6e4f2a1b", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := oldIDPattern.MatchString(tt.id)
			if result != tt.expected {
				t.Errorf("oldIDPattern.MatchString(%q) = %v, expected %v", tt.id, result, tt.expected)
			}
		})
	}
}
