package core

import (
	"testing"
)

func TestIsStatusTransitionValid(t *testing.T) {
	tests := []struct {
		name     string
		from     PostStatus
		to       PostStatus
		expected bool
	}{
		{"idea to draft", StatusIdea, StatusDraft, true},
		{"draft to ready", StatusDraft, StatusReady, true},
		{"ready to scheduled", StatusReady, StatusScheduled, true},
		{"scheduled to published", StatusScheduled, StatusPublished, true},
		{"idea to ready", StatusIdea, StatusReady, true},
		{"draft to published", StatusDraft, StatusPublished, true},
		{"published to draft", StatusPublished, StatusDraft, false},
		{"ready to draft", StatusReady, StatusDraft, false},
		{"same status", StatusDraft, StatusDraft, false},
		{"invalid status", PostStatus("invalid"), StatusDraft, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsStatusTransitionValid(tt.from, tt.to)
			if result != tt.expected {
				t.Errorf("IsStatusTransitionValid(%s, %s) = %v, expected %v", tt.from, tt.to, result, tt.expected)
			}
		})
	}
}

func TestPostStatusConstants(t *testing.T) {
	expected := []PostStatus{
		StatusIdea,
		StatusDraft,
		StatusReady,
		StatusScheduled,
		StatusPublished,
	}

	for i, status := range StatusOrder {
		if status != expected[i] {
			t.Errorf("StatusOrder[%d] = %s, expected %s", i, status, expected[i])
		}
	}
}

func TestPlatformConstants(t *testing.T) {
	if PlatformTelegram != "telegram" {
		t.Errorf("PlatformTelegram = %s, expected 'telegram'", PlatformTelegram)
	}
}
