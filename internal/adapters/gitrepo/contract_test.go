package gitrepo

import (
	"testing"

	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/adapters/fsrepo"
	"github.com/jtprogru/jtpost/internal/adapters/repotest"
	"github.com/jtprogru/jtpost/internal/core"
)

// TestGitFS_RunContract запускает общий контракт-сьют через GitDecorator над fsrepo.
// Все subtests должны проходить — decorator pass-through всё CRUD.
func TestGitFS_RunContract(t *testing.T) {
	repotest.RunContract(t, func(t *testing.T) (core.PostRepository, repotest.Capabilities, func()) {
		t.Helper()
		dir := t.TempDir()
		inner, err := fsrepo.NewFileSystemRepository(dir)
		if err != nil {
			t.Fatalf("fsrepo: %v", err)
		}
		dec, err := NewGitDecorator(inner, dir, config.GitStorageConfig{
			Enabled: true,
			Branch:  "main",
		})
		if err != nil {
			t.Fatalf("decorator: %v", err)
		}
		return dec, repotest.Capabilities{
			OptimisticLock: false,
			Transactions:   false,
		}, func() {}
	})
}
