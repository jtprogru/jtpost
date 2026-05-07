package logger

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestLogger_LevelString(t *testing.T) {
	tests := []struct {
		level  Level
		expect string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
		{Level(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expect, func(t *testing.T) {
			if got := tt.level.String(); got != tt.expect {
				t.Errorf("ожидаемый %q, получен %q", tt.expect, got)
			}
		})
	}
}

func TestParseFormat(t *testing.T) {
	cases := map[string]Format{
		"":       FormatText,
		"text":   FormatText,
		"TEXT":   FormatText,
		" json ": FormatJSON,
		"json":   FormatJSON,
		"JSON":   FormatJSON,
		"foo":    FormatText, // неизвестный формат → text
	}
	for in, want := range cases {
		if got := ParseFormat(in); got != want {
			t.Errorf("ParseFormat(%q) = %q, ожидали %q", in, got, want)
		}
	}
}

func TestLogger_New(t *testing.T) {
	t.Run("создание с конфигурацией", func(t *testing.T) {
		var buf bytes.Buffer
		log := New(Config{
			Output: &buf,
			Level:  LevelDebug,
			Prefix: "test",
			Debug:  true,
		})
		if log == nil {
			t.Fatal("логгер не создан")
		}
	})

	t.Run("создание по умолчанию", func(t *testing.T) {
		log := NewDefault()
		if log == nil {
			t.Fatal("логгер не создан")
		}
		if log.level != LevelInfo {
			t.Errorf("ожидаемый уровень Info, получен %v", log.level)
		}
	})

	t.Run("nil Output использует os.Stdout", func(t *testing.T) {
		log := New(Config{})
		if log == nil {
			t.Fatal("логгер не создан")
		}
	})
}

func TestLogger_SetLevel(t *testing.T) {
	log := NewDefault()

	log.SetLevel(LevelDebug)
	if log.level != LevelDebug {
		t.Errorf("ожидаемый уровень Debug, получен %v", log.level)
	}

	log.SetLevel(LevelError)
	if log.level != LevelError {
		t.Errorf("ожидаемый уровень Error, получен %v", log.level)
	}
}

func TestLogger_SetDebug(t *testing.T) {
	log := NewDefault()

	if log.debug {
		t.Error("debug должен быть выключен по умолчанию")
	}

	log.SetDebug(true)
	if !log.debug {
		t.Error("debug должен быть включён")
	}
	if log.level != LevelDebug {
		t.Errorf("ожидаемый уровень Debug, получен %v", log.level)
	}

	log.SetDebug(false)
	if log.debug {
		t.Error("debug должен быть выключен")
	}
}

func TestLogger_TextOutput(t *testing.T) {
	var buf bytes.Buffer
	log := New(Config{
		Output: &buf,
		Level:  LevelDebug,
		Format: FormatText,
	})

	cases := []struct {
		fn      func(string, ...any)
		level   string
		message string
	}{
		{log.Info, "INFO", "test message"},
		{log.Debug, "DEBUG", "debug message"},
		{log.Warn, "WARN", "warn message"},
		{log.Error, "ERROR", "error message"},
	}
	for _, tc := range cases {
		t.Run(tc.level, func(t *testing.T) {
			buf.Reset()
			tc.fn(tc.message)
			out := buf.String()
			if !strings.Contains(out, "level="+tc.level) {
				t.Errorf("ожидали level=%s в выводе, получили: %s", tc.level, out)
			}
			if !strings.Contains(out, tc.message) {
				t.Errorf("ожидали %q в выводе, получили: %s", tc.message, out)
			}
		})
	}
}

func TestLogger_JSONOutput(t *testing.T) {
	var buf bytes.Buffer
	log := New(Config{
		Output: &buf,
		Level:  LevelInfo,
		Format: FormatJSON,
	})
	log.Info("hello %s", "world")

	var rec map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &rec); err != nil {
		t.Fatalf("ожидали валидный JSON, получили %q: %v", buf.String(), err)
	}
	if rec["level"] != "INFO" {
		t.Errorf("ожидали level=INFO, получили %v", rec["level"])
	}
	if rec["msg"] != "hello world" {
		t.Errorf("ожидали msg='hello world', получили %v", rec["msg"])
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	log := New(Config{Output: &buf, Level: LevelWarn})

	t.Run("Debug не логируется при уровне Warn", func(t *testing.T) {
		buf.Reset()
		log.Debug("debug message")
		if buf.Len() > 0 {
			t.Errorf("ожидается пустой вывод, получено: %s", buf.String())
		}
	})
	t.Run("Info не логируется при уровне Warn", func(t *testing.T) {
		buf.Reset()
		log.Info("info message")
		if buf.Len() > 0 {
			t.Errorf("ожидается пустой вывод, получено: %s", buf.String())
		}
	})
	t.Run("Warn логируется при уровне Warn", func(t *testing.T) {
		buf.Reset()
		log.Warn("warn message")
		if buf.Len() == 0 {
			t.Error("ожидается вывод сообщения")
		}
	})
	t.Run("Error логируется при уровне Warn", func(t *testing.T) {
		buf.Reset()
		log.Error("error message")
		if buf.Len() == 0 {
			t.Error("ожидается вывод сообщения")
		}
	})
}

func TestLogger_Formatting(t *testing.T) {
	var buf bytes.Buffer
	log := New(Config{Output: &buf, Level: LevelDebug})

	t.Run("Infof форматирует сообщение", func(t *testing.T) {
		buf.Reset()
		log.Infof("user %s logged in, count: %d", "alice", 42)
		if !strings.Contains(buf.String(), "user alice logged in, count: 42") {
			t.Errorf("лог должен содержать отформатированное сообщение, получено: %s", buf.String())
		}
	})

	t.Run("Debugf форматирует сообщение", func(t *testing.T) {
		buf.Reset()
		log.Debugf("value: %v", []int{1, 2, 3})
		if !strings.Contains(buf.String(), "value: [1 2 3]") {
			t.Errorf("лог должен содержать отформатированное сообщение, получено: %s", buf.String())
		}
	})
}

func TestLogger_Prefix(t *testing.T) {
	var buf bytes.Buffer
	log := New(Config{Output: &buf, Level: LevelInfo, Prefix: "API"})
	log.Info("test message")
	if !strings.Contains(buf.String(), "component=API") {
		t.Errorf("лог должен содержать component=API, получено: %s", buf.String())
	}
}

func TestLogger_Concurrent(_ *testing.T) {
	var buf bytes.Buffer
	log := New(Config{Output: &buf, Level: LevelDebug})

	done := make(chan bool, 10)
	for i := range 10 {
		go func(id int) {
			for range 10 {
				log.Infof("goroutine %d, message", id)
			}
			done <- true
		}(i)
	}
	for range 10 {
		<-done
	}
}
