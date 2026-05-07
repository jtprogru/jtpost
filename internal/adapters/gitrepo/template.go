// Package gitrepo предоставляет Decorator над core.PostRepository, который
// после успешных мутаций (Create/Update/Delete) делает git add+commit, а
// при storage.git.auto_push=true — git push.
package gitrepo

import (
	"bytes"
	"errors"
	"fmt"
	"text/template"
	"time"

	"github.com/jtprogru/jtpost/internal/core"
)

const defaultCommitTemplate = "chore: {{.Operation}} post {{.Slug}}"

// TemplateVars — переменные доступные внутри commit-template.
type TemplateVars struct {
	Slug      string
	Title     string
	ID        string
	Status    string
	Operation string // "create" | "update" | "delete"
	Time      time.Time
}

// parseCommitTemplate парсит s или дефолтный шаблон если s == "".
// На ошибке оборачивает в core.ErrConfigInvalid.
func parseCommitTemplate(s string) (*template.Template, error) {
	if s == "" {
		s = defaultCommitTemplate
	}
	tmpl, err := template.New("commit").Parse(s)
	if err != nil {
		return nil, errors.Join(core.ErrConfigInvalid, err)
	}
	return tmpl, nil
}

// renderMessage применяет шаблон к (op, post). На runtime-ошибке возвращает
// fallback "chore: <op> post <slug>".
func renderMessage(tmpl *template.Template, op string, post *core.Post) string {
	vars := TemplateVars{
		Slug:      post.Slug,
		Title:     post.Title,
		ID:        post.ID.String(),
		Status:    string(post.Status),
		Operation: op,
		Time:      time.Now().UTC(),
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return fmt.Sprintf("chore: %s post %s", op, post.Slug)
	}
	return buf.String()
}
