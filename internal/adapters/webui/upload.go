package webui

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jtprogru/jtpost/internal/core"
)

// uploadDefaultMaxSize — fallback если в cfg не задан Upload.MaxSizeBytes.
const uploadDefaultMaxSize = int64(10 * 1024 * 1024)

// extForMIME — расширение для имени сохраняемого файла.
func extForMIME(mime string) (string, bool) {
	switch mime {
	case "image/jpeg":
		return ".jpg", true
	case "image/png":
		return ".png", true
	case "image/webp":
		return ".webp", true
	case "image/gif":
		return ".gif", true
	}
	return "", false
}

// handleUpload — POST /ui/upload (multipart "file"). Сохраняет файл,
// возвращает JSON {"url": "...", "markdown": "![](...)"} для inserter.
func (h *Handler) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.cfg == nil {
		http.Error(w, "config not loaded", http.StatusServiceUnavailable)
		return
	}
	maxSize := h.cfg.Server.Upload.MaxSizeBytes
	if maxSize <= 0 {
		maxSize = uploadDefaultMaxSize
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxSize+512)
	if err := r.ParseMultipartForm(maxSize + 512); err != nil {
		http.Error(w, "файл слишком большой или multipart-форма повреждена", http.StatusRequestEntityTooLarge)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "поле file обязательно", http.StatusBadRequest)
		return
	}
	defer func() { _ = file.Close() }()
	if header.Size > maxSize {
		http.Error(w, "файл превышает лимит", http.StatusRequestEntityTooLarge)
		return
	}

	// Sniff content-type по первым 512 байтам — не доверяем заголовку клиента.
	head := make([]byte, 512)
	n, _ := io.ReadFull(file, head)
	mime := http.DetectContentType(head[:n])
	if !mimeAllowed(h.cfg.Server.Upload.AllowedMIME, mime) {
		http.Error(w, "тип файла не разрешён: "+mime, http.StatusUnsupportedMediaType)
		return
	}
	ext, ok := extForMIME(mime)
	if !ok {
		http.Error(w, "неподдерживаемое расширение для "+mime, http.StatusUnsupportedMediaType)
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		http.Error(w, "rewind failed", http.StatusInternalServerError)
		return
	}

	dir := h.cfg.Server.Upload.Dir
	if dir == "" {
		dir = "data/uploads"
	}
	now := time.Now().UTC()
	subdir := filepath.Join(dir, fmt.Sprintf("%04d", now.Year()), fmt.Sprintf("%02d", int(now.Month())))
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		h.log.Error("ui upload mkdir: %v", err)
		http.Error(w, "не удалось создать каталог", http.StatusInternalServerError)
		return
	}
	name, err := randomName(16)
	if err != nil {
		http.Error(w, "rand failed", http.StatusInternalServerError)
		return
	}
	relPath := filepath.Join(fmt.Sprintf("%04d", now.Year()), fmt.Sprintf("%02d", int(now.Month())), name+ext)
	absPath := filepath.Join(dir, relPath)
	if err := saveFile(absPath, file, maxSize); err != nil {
		h.log.Error("ui upload save: %v", err)
		http.Error(w, "не удалось сохранить файл", http.StatusInternalServerError)
		return
	}

	// URL для веб-доступа — слешами, не os.PathSeparator.
	urlPath := "/ui/uploads/" + strings.ReplaceAll(relPath, string(os.PathSeparator), "/")
	alt := strings.TrimSuffix(filepath.Base(header.Filename), filepath.Ext(header.Filename))
	if alt == "" {
		alt = "image"
	}
	md := fmt.Sprintf("![%s](%s)", alt, urlPath)

	if h.auditSvc != nil {
		_ = h.auditSvc.Log(r.Context(), core.AuditEntry{
			Action:       core.AuditImageUploaded,
			Outcome:      core.AuditOutcomeSuccess,
			ResourceType: "image",
			ResourceID:   relPath,
			Metadata:     map[string]any{"via": "webui", "mime": mime, "size": header.Size},
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"url":      urlPath,
		"markdown": md,
	})
}

// handleUploadServe — GET /ui/uploads/{path} → отдаёт файл с диска.
// Защита: пути нормализуются и проверяются на выход за пределы базового каталога.
func (h *Handler) handleUploadServe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.cfg == nil {
		http.NotFound(w, r)
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/ui/uploads/")
	if rest == "" || strings.Contains(rest, "..") {
		http.NotFound(w, r)
		return
	}
	dir := h.cfg.Server.Upload.Dir
	if dir == "" {
		dir = "data/uploads"
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	target := filepath.Join(absDir, filepath.FromSlash(rest))
	if !strings.HasPrefix(target, absDir+string(os.PathSeparator)) {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, target)
}

func mimeAllowed(allowed []string, mime string) bool {
	if len(allowed) == 0 {
		return false
	}
	for _, a := range allowed {
		if strings.EqualFold(a, mime) {
			return true
		}
	}
	return false
}

func randomName(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// saveFile — копирует src в path с лимитом по размеру.
func saveFile(path string, src io.Reader, maxSize int64) error {
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	limited := io.LimitReader(src, maxSize+1)
	written, err := io.Copy(out, limited)
	if err != nil {
		return err
	}
	if written > maxSize {
		_ = os.Remove(path)
		return errors.New("file exceeds max size")
	}
	return nil
}
