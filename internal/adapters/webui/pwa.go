package webui

import (
	"net/http"
)

// handleServiceWorker отдаёт sw.js из embed-ассетов под путём /ui/sw.js,
// чтобы default scope service worker'а был "/ui/" (а не "/ui/static/").
// Альтернативой был бы header Service-Worker-Allowed, но статический mux
// не позволяет его выставить — отсюда отдельный handler.
func (h *Handler) handleServiceWorker(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data, err := staticFS.ReadFile("static/sw.js")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Service-Worker-Allowed", "/ui/")
	_, _ = w.Write(data)
}
