package webui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jtprogru/jtpost/internal/core"
)

// sseHeartbeat — частота keep-alive comment-фрейма.
const sseHeartbeat = 25 * time.Second

// handleEvents — GET /ui/events — Server-Sent Events stream.
// Соединение остаётся открытым; фреймы шлются при наличии события на bus
// или каждые sseHeartbeat секунд (для предотвращения idle-таймаутов прокси).
func (h *Handler) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.bus == nil {
		http.Error(w, "events disabled", http.StatusServiceUnavailable)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Снимаем write deadline, который http.Server.WriteTimeout мог поставить —
	// иначе долгоживущее SSE-соединение закроется через 15s.
	if rc := http.NewResponseController(w); rc != nil {
		_ = rc.SetWriteDeadline(time.Time{})
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // nginx: отключить буферизацию
	w.WriteHeader(http.StatusOK)

	// Сразу шлём комментарий — браузер сразу узнаёт, что соединение живо.
	_, _ = fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	ch, cancel := h.bus.Subscribe()
	defer cancel()
	ticker := time.NewTicker(sseHeartbeat)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case evt, ok := <-ch:
			if !ok {
				return
			}
			payload, err := json.Marshal(map[string]any{
				"topic":    evt.Topic,
				"occur_at": evt.OccurAt.UTC().Format(time.RFC3339),
				"data":     evt.Data,
			})
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evt.Topic, payload); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// publish — nil-safe shortcut для публикации события на bus.
func (h *Handler) publish(topic string, data map[string]any) {
	if h.bus == nil {
		return
	}
	h.bus.Publish(core.Event{Topic: topic, Data: data})
}
