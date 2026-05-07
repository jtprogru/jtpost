package core

import (
	"testing"
)

// TestIsTransitionAllowed_Table проверяет полный декартов перебор статусов
// против allowedTransitions.
//
// Property: CP-4 (TransitionTableClosure).
func TestIsTransitionAllowed_Table(t *testing.T) {
	allowed := map[[2]PostStatus]bool{
		{StatusIdea, StatusDraft}:          true,
		{StatusDraft, StatusReady}:         true,
		{StatusReady, StatusScheduled}:     true,
		{StatusReady, StatusPublished}:     true,
		{StatusScheduled, StatusPublished}: true,
		{StatusScheduled, StatusReady}:     true,
		{StatusScheduled, StatusFailed}:    true,
		{StatusFailed, StatusReady}:        true,
		{StatusFailed, StatusArchived}:     true,
		{StatusPublished, StatusArchived}:  true,
	}

	all := AllStatuses()
	count := 0
	for _, from := range all {
		for _, to := range all {
			expected := allowed[[2]PostStatus{from, to}]
			got := IsTransitionAllowed(from, to)
			if got != expected {
				t.Errorf("IsTransitionAllowed(%s, %s) = %v, want %v", from, to, got, expected)
			}
			if got {
				count++
			}
		}
	}

	if count != len(allowed) {
		t.Errorf("expected exactly %d allowed transitions, got %d", len(allowed), count)
	}
}

// TestIsTransitionAllowed_UnknownStatus проверяет, что неизвестные статусы не
// дают разрешённых переходов.
func TestIsTransitionAllowed_UnknownStatus(t *testing.T) {
	tests := []struct {
		name string
		from PostStatus
		to   PostStatus
	}{
		{"unknown from", PostStatus("unknown"), StatusDraft},
		{"unknown to", StatusIdea, PostStatus("unknown")},
		{"both unknown", PostStatus("a"), PostStatus("b")},
		{"same status", StatusDraft, StatusDraft},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if IsTransitionAllowed(tt.from, tt.to) {
				t.Errorf("IsTransitionAllowed(%q, %q) returned true, want false", tt.from, tt.to)
			}
		})
	}
}

// TestIsStatusTransitionValid_DeprecatedAlias подтверждает, что deprecated alias
// идентичен IsTransitionAllowed.
func TestIsStatusTransitionValid_DeprecatedAlias(t *testing.T) {
	for _, from := range AllStatuses() {
		for _, to := range AllStatuses() {
			if IsStatusTransitionValid(from, to) != IsTransitionAllowed(from, to) {
				t.Errorf("alias mismatch for (%s, %s)", from, to)
			}
		}
	}
}

// TestAllStatuses_Count проверяет, что всего 7 статусов.
func TestAllStatuses_Count(t *testing.T) {
	if got := len(AllStatuses()); got != 7 {
		t.Errorf("len(AllStatuses()) = %d, want 7", got)
	}
}

// TestPostStatusConstants проверяет значения констант.
func TestPostStatusConstants(t *testing.T) {
	tests := []struct {
		s     PostStatus
		value string
	}{
		{StatusIdea, "idea"},
		{StatusDraft, "draft"},
		{StatusReady, "ready"},
		{StatusScheduled, "scheduled"},
		{StatusPublished, "published"},
		{StatusArchived, "archived"},
		{StatusFailed, "failed"},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			if string(tt.s) != tt.value {
				t.Errorf("status %q != %q", string(tt.s), tt.value)
			}
		})
	}
}
