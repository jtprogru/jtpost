package gitrepo

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/core"
)

func makePost(slug string) *core.Post {
	return &core.Post{
		ID:     core.PostID(uuid.MustParse("01900000-0000-7000-8000-000000000001")),
		Slug:   slug,
		Title:  "T-" + slug,
		Status: core.StatusDraft,
	}
}

func TestParseCommitTemplate_Default(t *testing.T) {
	tmpl, err := parseCommitTemplate("")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	got := renderMessage(tmpl, "create", makePost("hello"))
	if got != "chore: create post hello" {
		t.Errorf("got %q", got)
	}
}

func TestParseCommitTemplate_Valid(t *testing.T) {
	tmpl, err := parseCommitTemplate("post: {{.Title}} ({{.ID}}) — {{.Status}}/{{.Operation}}")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	post := makePost("x")
	got := renderMessage(tmpl, "update", post)
	for _, want := range []string{"T-x", "01900000", "draft", "update"} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered %q does not contain %q", got, want)
		}
	}
}

func TestParseCommitTemplate_Invalid(t *testing.T) {
	tt := []string{
		"{{.Slug",
		"{{.Foo}}{{",
		"{{ if .Slug }}",
	}
	for _, tc := range tt {
		t.Run(tc, func(t *testing.T) {
			_, err := parseCommitTemplate(tc)
			if !errors.Is(err, core.ErrConfigInvalid) {
				t.Errorf("want ErrConfigInvalid, got %v", err)
			}
		})
	}
}

func TestRenderMessage_AllVars(t *testing.T) {
	tmpl, err := parseCommitTemplate("{{.Slug}}|{{.Title}}|{{.ID}}|{{.Status}}|{{.Operation}}|{{.Time.Year}}")
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range []string{"create", "update", "delete"} {
		t.Run(op, func(t *testing.T) {
			got := renderMessage(tmpl, op, makePost("s"))
			if !strings.Contains(got, op) {
				t.Errorf("operation %q missing in %q", op, got)
			}
			if !strings.Contains(got, "s|T-s|") {
				t.Errorf("slug/title missing in %q", got)
			}
		})
	}
}

func TestGitAuthor_FromEnv(t *testing.T) {
	t.Setenv("GIT_AUTHOR_NAME", "Alice")
	t.Setenv("GIT_AUTHOR_EMAIL", "alice@example.com")
	a := gitAuthor()
	if a.Name != "Alice" || a.Email != "alice@example.com" {
		t.Errorf("got %s/%s", a.Name, a.Email)
	}
}

func TestGitAuthor_Fallback(t *testing.T) {
	t.Setenv("GIT_AUTHOR_NAME", "")
	t.Setenv("GIT_AUTHOR_EMAIL", "")
	a := gitAuthor()
	if a.Name != "jtpost" || a.Email != "bot@jtpost.local" {
		t.Errorf("got %s/%s", a.Name, a.Email)
	}
}
