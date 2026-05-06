// Package logger предоставляет логгер на базе log/slog с поддержкой text и json форматов.
package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

// Level уровень логирования.
type Level int

const (
	// LevelDebug отладочные сообщения.
	LevelDebug Level = iota
	// LevelInfo информационные сообщения.
	LevelInfo
	// LevelWarn предупреждения.
	LevelWarn
	// LevelError ошибки.
	LevelError
)

// String возвращает строковое представление уровня.
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

func (l Level) toSlog() slog.Level {
	switch l {
	case LevelDebug:
		return slog.LevelDebug
	case LevelInfo:
		return slog.LevelInfo
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	}
	return slog.LevelInfo
}

// Format формат вывода логов.
type Format string

const (
	// FormatText человекочитаемый текстовый формат (по умолчанию).
	FormatText Format = "text"
	// FormatJSON структурированные JSON-логи (по одной записи на строку).
	FormatJSON Format = "json"
)

// ParseFormat разбирает строку в Format. Пустое значение и неизвестные значения
// дают FormatText.
func ParseFormat(s string) Format {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "json":
		return FormatJSON
	default:
		return FormatText
	}
}

// Logger обёртка над *slog.Logger c fmt-подобным API для сохранения
// совместимости со старыми вызовами по всему проекту.
type Logger struct {
	mu     sync.Mutex
	out    io.Writer
	level  Level
	debug  bool
	prefix string
	lvl    *slog.LevelVar
	logger *slog.Logger
}

// Config конфигурация логгера.
type Config struct {
	// Output выходной поток (по умолчанию os.Stdout).
	Output io.Writer
	// Level минимальный уровень логирования.
	Level Level
	// Prefix добавляется как атрибут "component" к каждой записи.
	Prefix string
	// Debug включает уровень DEBUG (перекрывает Level).
	Debug bool
	// Format формат вывода: "text" (по умолчанию) или "json".
	Format Format
}

// New создаёт новый Logger.
func New(cfg Config) *Logger {
	out := cfg.Output
	if out == nil {
		out = os.Stdout
	}
	level := cfg.Level
	if cfg.Debug {
		level = LevelDebug
	}

	lvl := new(slog.LevelVar)
	lvl.Set(level.toSlog())
	opts := &slog.HandlerOptions{Level: lvl}

	var h slog.Handler
	switch cfg.Format {
	case FormatJSON:
		h = slog.NewJSONHandler(out, opts)
	default:
		h = slog.NewTextHandler(out, opts)
	}

	sl := slog.New(h)
	if cfg.Prefix != "" {
		sl = sl.With(slog.String("component", cfg.Prefix))
	}

	return &Logger{
		out:    out,
		level:  level,
		debug:  cfg.Debug,
		prefix: cfg.Prefix,
		lvl:    lvl,
		logger: sl,
	}
}

// NewDefault создаёт логгер по умолчанию (INFO, text).
func NewDefault() *Logger {
	return New(Config{Level: LevelInfo})
}

// SetLevel динамически меняет минимальный уровень логирования.
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
	l.lvl.Set(level.toSlog())
}

// SetDebug включает/выключает режим отладки.
// Включение поднимает минимальный уровень до DEBUG; выключение оставляет
// текущий уровень нетронутым (поведение из старой реализации).
func (l *Logger) SetDebug(debug bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.debug = debug
	if debug {
		l.level = LevelDebug
		l.lvl.Set(slog.LevelDebug)
	}
}

func (l *Logger) write(level slog.Level, format string, args ...any) {
	ctx := context.Background()
	if !l.logger.Enabled(ctx, level) {
		return
	}
	msg := fmt.Sprintf(format, args...)
	l.logger.Log(ctx, level, msg)
}

// Debug логирует отладочное сообщение.
func (l *Logger) Debug(format string, args ...any) { l.write(slog.LevelDebug, format, args...) }

// Debugf — алиас Debug, оставлен для обратной совместимости.
func (l *Logger) Debugf(format string, args ...any) { l.write(slog.LevelDebug, format, args...) }

// Info логирует информационное сообщение.
func (l *Logger) Info(format string, args ...any) { l.write(slog.LevelInfo, format, args...) }

// Infof — алиас Info, оставлен для обратной совместимости.
func (l *Logger) Infof(format string, args ...any) { l.write(slog.LevelInfo, format, args...) }

// Warn логирует предупреждение.
func (l *Logger) Warn(format string, args ...any) { l.write(slog.LevelWarn, format, args...) }

// Warnf — алиас Warn, оставлен для обратной совместимости.
func (l *Logger) Warnf(format string, args ...any) { l.write(slog.LevelWarn, format, args...) }

// Error логирует ошибку.
func (l *Logger) Error(format string, args ...any) { l.write(slog.LevelError, format, args...) }

// Errorf — алиас Error, оставлен для обратной совместимости.
func (l *Logger) Errorf(format string, args ...any) { l.write(slog.LevelError, format, args...) }
