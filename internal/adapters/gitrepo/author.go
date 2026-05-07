package gitrepo

import (
	"os"
	"time"

	"github.com/go-git/go-git/v5/plumbing/object"
)

// gitAuthor возвращает identity для git-коммитов.
// Приоритет: env GIT_AUTHOR_NAME / GIT_AUTHOR_EMAIL → fallback "jtpost" / "bot@jtpost.local".
func gitAuthor() *object.Signature {
	name := os.Getenv("GIT_AUTHOR_NAME")
	if name == "" {
		name = "jtpost"
	}
	email := os.Getenv("GIT_AUTHOR_EMAIL")
	if email == "" {
		email = "bot@jtpost.local"
	}
	return &object.Signature{
		Name:  name,
		Email: email,
		When:  time.Now().UTC(),
	}
}
