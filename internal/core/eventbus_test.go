package core

import (
	"sync"
	"testing"
	"time"
)

func TestMemoryBus_PublishSubscribe(t *testing.T) {
	t.Parallel()
	bus := NewMemoryBus(4)
	ch, cancel := bus.Subscribe()
	defer cancel()
	bus.Publish(Event{Topic: "post.changed"})
	select {
	case e := <-ch:
		if e.Topic != "post.changed" {
			t.Errorf("topic=%q", e.Topic)
		}
		if e.OccurAt.IsZero() {
			t.Error("OccurAt должен быть установлен автоматически")
		}
	case <-time.After(time.Second):
		t.Fatal("event not delivered")
	}
}

func TestMemoryBus_MultipleSubscribers(t *testing.T) {
	t.Parallel()
	bus := NewMemoryBus(4)
	ch1, c1 := bus.Subscribe()
	ch2, c2 := bus.Subscribe()
	defer c1()
	defer c2()
	bus.Publish(Event{Topic: "x"})
	for _, ch := range []<-chan Event{ch1, ch2} {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatal("event not delivered to subscriber")
		}
	}
}

func TestMemoryBus_CancelClosesChannel(t *testing.T) {
	t.Parallel()
	bus := NewMemoryBus(2)
	ch, cancel := bus.Subscribe()
	cancel()
	if _, ok := <-ch; ok {
		t.Error("expected channel closed after cancel")
	}
	cancel() // idempotent
}

func TestMemoryBus_SlowConsumerDrops(t *testing.T) {
	t.Parallel()
	bus := NewMemoryBus(1)
	_, cancel := bus.Subscribe()
	defer cancel()
	// 5 publish без чтения — bus не должен блокироваться.
	done := make(chan struct{})
	go func() {
		for range 5 {
			bus.Publish(Event{Topic: "spam"})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("publish blocked on slow consumer")
	}
}

func TestMemoryBus_ConcurrentSubscribePublish(t *testing.T) {
	t.Parallel()
	bus := NewMemoryBus(8)
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			_, cancel := bus.Subscribe()
			defer cancel()
			bus.Publish(Event{Topic: "race"})
		})
	}
	wg.Wait()
}
