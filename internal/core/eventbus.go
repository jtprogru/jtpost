package core

import (
	"sync"
	"time"
)

// Event — payload, доставляемый подписчикам.
type Event struct {
	Topic     string
	OccurAt   time.Time
	TenantID  string         // опционально — для фильтрации по tenant
	Data      map[string]any // произвольные поля события
}

// EventBus — мини-pub/sub для real-time обновлений UI.
//
// Реализация может быть синхронной (для тестов) или асинхронной
// (in-memory с буферизованными каналами). Подписка возвращает канал
// чтения и cancel-функцию, которая закрывает канал и отписывает
// подписчика. Cancel должен быть идемпотентным.
type EventBus interface {
	Publish(evt Event)
	Subscribe() (<-chan Event, func())
}

// MemoryBus — потокобезопасный in-memory bus с buffered-каналами.
// При переполнении канала подписчика событие для него дропается
// (slow-consumer не блокирует publisher).
type MemoryBus struct {
	mu      sync.RWMutex
	subs    map[int]chan Event
	nextID  int
	bufSize int
}

// NewMemoryBus создаёт новый MemoryBus с указанным размером буфера
// per-subscriber. Если bufSize <= 0, используется 16.
func NewMemoryBus(bufSize int) *MemoryBus {
	if bufSize <= 0 {
		bufSize = 16
	}
	return &MemoryBus{
		subs:    make(map[int]chan Event),
		bufSize: bufSize,
	}
}

// Publish рассылает событие всем подписчикам не-блокирующе.
func (b *MemoryBus) Publish(evt Event) {
	if evt.OccurAt.IsZero() {
		evt.OccurAt = time.Now()
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subs {
		select {
		case ch <- evt:
		default:
			// Slow consumer — drop, чтобы не блокировать publisher.
		}
	}
}

// Subscribe регистрирует нового подписчика.
func (b *MemoryBus) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, b.bufSize)
	b.mu.Lock()
	id := b.nextID
	b.nextID++
	b.subs[id] = ch
	b.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			b.mu.Lock()
			if c, ok := b.subs[id]; ok {
				delete(b.subs, id)
				close(c)
			}
			b.mu.Unlock()
		})
	}
	return ch, cancel
}
