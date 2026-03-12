package logger

import (
	"bytes"
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

func TestLogger_Output(t *testing.T) {
	var buf bytes.Buffer
	log := New(Config{
		Output: &buf,
		Level:  LevelDebug,
	})

	t.Run("Info логирует сообщение", func(t *testing.T) {
		buf.Reset()
		log.Info("test message")

		output := buf.String()
		if !strings.Contains(output, "[INFO]") {
			t.Errorf("лог должен содержать '[INFO]', получено: %s", output)
		}
		if !strings.Contains(output, "test message") {
			t.Errorf("лог должен содержать 'test message', получено: %s", output)
		}
	})

	t.Run("Debug логирует сообщение", func(t *testing.T) {
		buf.Reset()
		log.Debug("debug message")

		output := buf.String()
		if !strings.Contains(output, "[DEBUG]") {
			t.Errorf("лог должен содержать '[DEBUG]', получено: %s", output)
		}
		if !strings.Contains(output, "debug message") {
			t.Errorf("лог должен содержать 'debug message', получено: %s", output)
		}
	})

	t.Run("Warn логирует сообщение", func(t *testing.T) {
		buf.Reset()
		log.Warn("warn message")

		output := buf.String()
		if !strings.Contains(output, "[WARN]") {
			t.Errorf("лог должен содержать '[WARN]', получено: %s", output)
		}
	})

	t.Run("Error логирует сообщение", func(t *testing.T) {
		buf.Reset()
		log.Error("error message")

		output := buf.String()
		if !strings.Contains(output, "[ERROR]") {
			t.Errorf("лог должен содержать '[ERROR]', получено: %s", output)
		}
	})
}

func TestLogger_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	log := New(Config{
		Output: &buf,
		Level:  LevelWarn, // только WARN и ERROR
	})

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
	log := New(Config{
		Output: &buf,
		Level:  LevelDebug, // Включаем DEBUG для тестов
	})

	t.Run("Infof форматирует сообщение", func(t *testing.T) {
		buf.Reset()
		log.Infof("user %s logged in, count: %d", "alice", 42)

		output := buf.String()
		if !strings.Contains(output, "user alice logged in, count: 42") {
			t.Errorf("лог должен содержать отформатированное сообщение, получено: %s", output)
		}
	})

	t.Run("Debugf форматирует сообщение", func(t *testing.T) {
		buf.Reset()
		log.Debugf("value: %v", []int{1, 2, 3})

		output := buf.String()
		if !strings.Contains(output, "value: [1 2 3]") {
			t.Errorf("лог должен содержать отформатированное сообщение, получено: %s", output)
		}
	})
}

func TestLogger_Prefix(t *testing.T) {
	var buf bytes.Buffer
	log := New(Config{
		Output: &buf,
		Level:  LevelInfo,
		Prefix: "[API]",
	})

	log.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "[API]") {
		t.Errorf("лог должен содержать префикс '[API]', получено: %s", output)
	}
}

func TestLogger_Concurrent(_ *testing.T) {
	var buf bytes.Buffer
	log := New(Config{
		Output: &buf,
		Level:  LevelDebug,
	})

	done := make(chan bool, 10)

	// Запускаем несколько горутин для одновременного логирования
	for i := range 10 {
		go func(id int) {
			for range 10 {
				log.Infof("goroutine %d, message", id)
			}
			done <- true
		}(i)
	}

	// Ждём завершения всех горутин
	for range 10 {
		<-done
	}

	// Если паники не произошло, тест пройден
}
