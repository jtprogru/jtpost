package telegram

import (
	"context"
	"testing"

	"github.com/jtprogru/jtpost/internal/core"
)

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				BotToken:  "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
				ChannelID: "@testchannel",
			},
			wantErr: false,
		},
		{
			name: "missing bot token",
			cfg: Config{
				BotToken:  "",
				ChannelID: "@testchannel",
			},
			wantErr: true,
		},
		{
			name: "missing channel ID",
			cfg: Config{
				BotToken:  "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
				ChannelID: "",
			},
			wantErr: true,
		},
		{
			name: "both missing",
			cfg: Config{
				BotToken:  "",
				ChannelID: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewPublisher(t *testing.T) {
	cfg := Config{
		BotToken:  "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
		ChannelID: "@testchannel",
	}

	publisher := NewPublisher(cfg)
	if publisher == nil {
		t.Fatal("NewPublisher() returned nil")
	}

	if publisher.botToken != cfg.BotToken {
		t.Errorf("botToken = %q, want %q", publisher.botToken, cfg.BotToken)
	}

	if publisher.channelID != cfg.ChannelID {
		t.Errorf("channelID = %q, want %q", publisher.channelID, cfg.ChannelID)
	}

	if publisher.client == nil {
		t.Error("client should not be nil")
	}
}

func TestPublisher_Platform(t *testing.T) {
	publisher := NewPublisher(Config{
		BotToken:  "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
		ChannelID: "@testchannel",
	})

	platform := publisher.Platform()
	if platform != "telegram" {
		t.Errorf("Platform() = %q, want %q", platform, "telegram")
	}
}

func TestPublisher_Publish_EmptyContent(t *testing.T) {
	publisher := NewPublisher(Config{
		BotToken:  "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
		ChannelID: "@testchannel",
	})

	post := &core.Post{
		ID:      "test-post",
		Title:   "Test Post",
		Content: "",
	}

	_, err := publisher.Publish(context.Background(), post)
	if err == nil {
		t.Error("Publish() expected error for empty content, got nil")
	}
}

func TestPtrTime(t *testing.T) {
	// Просто проверяем что функция работает
	testTime := core.SystemClock{}.Now()
	result := ptrTime(testTime)
	if result == nil {
		t.Fatal("ptrTime() returned nil")
	}
	if *result != testTime {
		t.Errorf("ptrTime() = %v, want %v", *result, testTime)
	}
}
