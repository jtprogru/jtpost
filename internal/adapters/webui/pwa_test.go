package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUI_ServiceWorker_ServedFromUIRoot(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/ui/sw.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "javascript") {
		t.Errorf("Content-Type=%q", rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("Service-Worker-Allowed") != "/ui/" {
		t.Errorf("missing Service-Worker-Allowed header")
	}
	body := rec.Body.String()
	if !strings.Contains(body, "addEventListener('install'") {
		t.Errorf("body doesn't look like sw.js: %s", body[:min(200, len(body))])
	}
}

func TestUI_ServiceWorker_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	req := httptest.NewRequest(http.MethodPost, "/ui/sw.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d, want 405", rec.Code)
	}
}

func TestUI_Manifest_ServedAndValidJSON(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/ui/static/manifest.webmanifest", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("manifest is not valid JSON: %v", err)
	}
	for _, k := range []string{"name", "start_url", "display", "icons"} {
		if _, ok := m[k]; !ok {
			t.Errorf("manifest missing %q", k)
		}
	}
	if m["start_url"] != "/ui/" {
		t.Errorf("start_url=%v, want /ui/", m["start_url"])
	}
}

func TestUI_Layout_LinksManifestAndSW(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/ui/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	for _, want := range []string{
		`rel="manifest"`,
		"manifest.webmanifest",
		`name="theme-color"`,
		"/ui/sw.js",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("layout missing %q", want)
		}
	}
}

func TestUI_Icons_Served(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	for _, p := range []string{"/ui/static/icon-192.png", "/ui/static/icon-512.png"} {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s status=%d", p, rec.Code)
			continue
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "image/png") {
			t.Errorf("%s Content-Type=%q", p, ct)
		}
	}
}
