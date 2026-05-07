package webui

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/core"
)

func newUploadHandler(t *testing.T) (*Handler, string) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.NewDefaultConfig()
	cfg.Server.Upload.Dir = dir
	cfg.Server.Upload.MaxSizeBytes = 1024 * 1024
	cfg.Server.Upload.AllowedMIME = []string{"image/png", "image/jpeg", "image/webp", "image/gif"}
	svc := core.NewPostService(&fakeRepo{}, core.SystemClock{})
	h := NewHandler(Config{Service: svc, Cfg: cfg})
	return h, dir
}

func encodePNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func multipartBody(t *testing.T, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(fw, bytes.NewReader(content)); err != nil {
		t.Fatal(err)
	}
	_ = mw.Close()
	return body, mw.FormDataContentType()
}

func TestUI_Upload_PNG_Success(t *testing.T) {
	t.Parallel()
	h, dir := newUploadHandler(t)
	body, ct := multipartBody(t, "hello.png", encodePNG(t))
	req := httptest.NewRequest(http.MethodPost, "/ui/upload", body)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v body=%s", err, rec.Body.String())
	}
	if !strings.HasPrefix(resp["url"], "/ui/uploads/") {
		t.Errorf("url=%q expected /ui/uploads/ prefix", resp["url"])
	}
	if !strings.HasPrefix(resp["markdown"], "![hello](/ui/uploads/") {
		t.Errorf("markdown=%q", resp["markdown"])
	}
	rel := strings.TrimPrefix(resp["url"], "/ui/uploads/")
	if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
		t.Errorf("file not on disk: %v", err)
	}
}

func TestUI_Upload_RejectsNonImage(t *testing.T) {
	t.Parallel()
	h, _ := newUploadHandler(t)
	body, ct := multipartBody(t, "evil.txt", []byte("plain text not image"))
	req := httptest.NewRequest(http.MethodPost, "/ui/upload", body)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status=%d, want 415; body=%s", rec.Code, rec.Body.String())
	}
}

func TestUI_Upload_RejectsTooLarge(t *testing.T) {
	t.Parallel()
	h, _ := newUploadHandler(t)
	h.cfg.Server.Upload.MaxSizeBytes = 100 // small
	big := make([]byte, 4096)
	body, ct := multipartBody(t, "big.png", big)
	req := httptest.NewRequest(http.MethodPost, "/ui/upload", body)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d, want 413", rec.Code)
	}
}

func TestUI_Upload_MissingFileField(t *testing.T) {
	t.Parallel()
	h, _ := newUploadHandler(t)
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	_ = mw.WriteField("other", "value")
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/ui/upload", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
}

func TestUI_Upload_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	h, _ := newUploadHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/ui/upload", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d, want 405", rec.Code)
	}
}

func TestUI_UploadServe_Success(t *testing.T) {
	t.Parallel()
	h, dir := newUploadHandler(t)
	// Положим файл напрямую и попробуем достать.
	rel := filepath.Join("2026", "05", "test.png")
	abs := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, encodePNG(t), 0o644); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/ui/uploads/2026/05/test.png", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestUI_UploadServe_RejectsTraversal(t *testing.T) {
	t.Parallel()
	h, _ := newUploadHandler(t)
	// http.ServeMux нормализует `..` в path и отдаёт 301/307 редирект на
	// очищенный URL — файл за пределами upload-каталога никогда не отдаётся.
	// Дополнительно проверяем явный traversal внутри handler через r.URL.Path.
	req := httptest.NewRequest(http.MethodGet, "/ui/uploads/", nil)
	req.URL.Path = "/ui/uploads/../../../etc/passwd"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatalf("traversal attempt must NOT return 200; got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestUI_Upload_NoConfig_503(t *testing.T) {
	t.Parallel()
	svc := core.NewPostService(&fakeRepo{}, core.SystemClock{})
	h := NewHandler(Config{Service: svc})
	body, ct := multipartBody(t, "x.png", encodePNG(t))
	req := httptest.NewRequest(http.MethodPost, "/ui/upload", body)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want 503", rec.Code)
	}
}
