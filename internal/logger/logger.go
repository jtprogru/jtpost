// Package logger предоставляет простой логгер с уровнями для jtpost.
package logger

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
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

// Logger простой логгер с уровнями.
type Logger struct {
	mu      sync.Mutex
	out     io.Writer
	level   Level
	prefix  string
	debug   bool
}

// Config конфигурация логгера.
type Config struct {
	// Output выходной поток (по умолчанию os.Stdout)
	Output io.Writer
	// Level минимальный уровень логирования
	Level Level
	// Prefix префикс для всех сообщений
	Prefix string
	// Debug режим отладки (включает DEBUG уровень)
	Debug bool
}

// New создаёт новый экземпляр Logger.
func New(cfg Config) *Logger {
	out := cfg.Output
	if out == nil {
		out = os.Stdout
	}

	level := cfg.Level
	if cfg.Debug {
		level = LevelDebug
	}

	return &Logger{
		out:    out,
		level:  level,
		prefix: cfg.Prefix,
		debug:  cfg.Debug,
	}
}

// NewDefault создаёт логгер по умолчанию (INFO уровень).
func NewDefault() *Logger {
	return New(Config{
		Level: LevelInfo,
	})
}

// SetLevel устанавливает новый уровень логирования.
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetDebug включает/выключает режим отладки.
func (l *Logger) SetDebug(debug bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.debug = debug
	if debug {
		l.level = LevelDebug
	}
}

// Debug логирует отладочное сообщение.
func (l *Logger) Debug(format string, args ...any) {
	l.log(LevelDebug, format, args...)
}

// Debugf логирует отладочное сообщение с форматированием.
func (l *Logger) Debugf(format string, args ...any) {
	l.log(LevelDebug, format, args...)
}

// Info логирует информационное сообщение.
func (l *Logger) Info(format string, args ...any) {
	l.log(LevelInfo, format, args...)
}

// Infof логирует информационное сообщение с форматированием.
func (l *Logger) Infof(format string, args ...any) {
	l.log(LevelInfo, format, args...)
}

// Warn логирует предупреждение.
func (l *Logger) Warn(format string, args ...any) {
	l.log(LevelWarn, format, args...)
}

// Warnf логирует предупреждение с форматированием.
func (l *Logger) Warnf(format string, args ...any) {
	l.log(LevelWarn, format, args...)
}

// Error логирует ошибку.
func (l *Logger) Error(format string, args ...any) {
	l.log(LevelError, format, args...)
}

// Errorf логирует ошибку с форматированием.
func (l *Logger) Errorf(format string, args ...any) {
	l.log(LevelError, format, args...)
}

// log внутренняя функция логирования.
func (l *Logger) log(level Level, format string, args ...any) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now().Format("2006-01-02 15:04:05.000")
	prefix := l.prefix
	if prefix != "" {
		prefix = " " + prefix
	}

	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(l.out, "[%s] [%s]%s %s\n", now, level.String(), prefix, msg)
}
