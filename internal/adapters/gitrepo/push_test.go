package gitrepo

import (
	"testing"

	"github.com/go-git/go-git/v5"
	gitcfg "github.com/go-git/go-git/v5/config"

	"github.com/jtprogru/jtpost/internal/adapters/config"
)

// newBareRemote создаёт bare git-репо в tempdir и возвращает его file://-URL.
func newBareRemote(t *testing.T) (url, dir string) {
	t.Helper()
	dir = t.TempDir()
	if _, err := git.PlainInit(dir, true); err != nil {
		t.Fatalf("PlainInit bare: %v", err)
	}
	return "file://" + dir, dir
}

func TestGitDecorator_Push_Success_BareRemote(t *testing.T) {
	remoteURL, bareDir := newBareRemote(t)

	d, _ := newDecorator(t, config.GitStorageConfig{
		Enabled:  true,
		AutoPush: true,
		Remote:   remoteURL,
		Branch:   "main",
	})

	// Зарегистрировать remote 'origin' в worktree-репо.
	if _, err := d.repo.CreateRemote(&gitcfg.RemoteConfig{
		Name: "origin",
		URLs: []string{remoteURL},
	}); err != nil {
		t.Fatalf("CreateRemote: %v", err)
	}

	post := mkPost("push-target")
	if err := d.Create(testCtx(), post); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Открыть bare-remote и проверить что коммит прилетел.
	bare, err := git.PlainOpen(bareDir)
	if err != nil {
		t.Fatalf("PlainOpen bare: %v", err)
	}
	if _, err := bare.Reference("refs/heads/main", false); err != nil {
		t.Errorf("bare remote should have refs/heads/main after push: %v", err)
	}
}

func TestGitDecorator_Push_Failed_NoOpReturn(t *testing.T) {
	d, _ := newDecorator(t, config.GitStorageConfig{
		Enabled:  true,
		AutoPush: true,
		Remote:   "file:///nonexistent/path/that/does/not/exist",
		Branch:   "main",
	})

	// Регистрируем тот же несуществующий remote.
	if _, err := d.repo.CreateRemote(&gitcfg.RemoteConfig{
		Name: "origin",
		URLs: []string{"file:///nonexistent/path/that/does/not/exist"},
	}); err != nil {
		t.Fatalf("CreateRemote: %v", err)
	}

	post := mkPost("push-fail-soft")
	// Push упадёт, но Create должен вернуть nil (REQ-4.3).
	if err := d.Create(testCtx(), post); err != nil {
		t.Fatalf("Create returned error despite soft-push-fail: %v", err)
	}

	// Локальный коммит должен существовать.
	head, err := d.repo.Head()
	if err != nil {
		t.Fatalf("Head: %v", err)
	}
	if head.Hash().IsZero() {
		t.Errorf("expected at least one commit locally")
	}
}
