package gitrepo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"

	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/core"
)

const (
	staleLockThreshold = 60 * time.Second
	pushTimeout        = 30 * time.Second
)

// GitDecorator оборачивает core.PostRepository, добавляя git-commit/push
// после успешных мутаций.
type GitDecorator struct {
	inner    core.PostRepository
	repo     *git.Repository
	postsDir string
	cfg      config.GitStorageConfig
	template *template.Template
	mu       sync.Mutex
	detached bool
}

// NewGitDecorator открывает или инициализирует git-репозиторий в postsDir
// и возвращает Decorator поверх inner.
func NewGitDecorator(inner core.PostRepository, postsDir string, cfg config.GitStorageConfig) (*GitDecorator, error) {
	tmpl, err := parseCommitTemplate(cfg.CommitTemplate)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(postsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create postsDir: %w", err)
	}

	if err := removeStaleLockfile(postsDir); err != nil {
		slog.Warn("git: failed to inspect lockfile", "err", err)
	}

	branch := cfg.Branch
	if branch == "" {
		branch = "main"
	}

	repo, err := git.PlainOpen(postsDir)
	if err != nil {
		if !errors.Is(err, git.ErrRepositoryNotExists) {
			return nil, fmt.Errorf("git open: %w", err)
		}
		repo, err = git.PlainInit(postsDir, false)
		if err != nil {
			return nil, fmt.Errorf("git init: %w", err)
		}
		if err := repo.Storer.SetReference(plumbing.NewSymbolicReference(
			plumbing.HEAD, plumbing.NewBranchReferenceName(branch),
		)); err != nil {
			return nil, fmt.Errorf("set initial branch: %w", err)
		}
	}

	detached := false
	headRef, headErr := repo.Reference(plumbing.HEAD, false)
	if headErr == nil && headRef != nil && headRef.Type() == plumbing.HashReference {
		// HEAD это hash-ref напрямую → detached.
		detached = true
		slog.Warn("git: detached HEAD detected, auto-commit disabled", "dir", postsDir)
	}

	return &GitDecorator{
		inner:    inner,
		repo:     repo,
		postsDir: postsDir,
		cfg:      cfg,
		template: tmpl,
		detached: detached,
	}, nil
}

// removeStaleLockfile удаляет .git/index.lock, если ему больше staleLockThreshold.
func removeStaleLockfile(postsDir string) error {
	lock := filepath.Join(postsDir, ".git", "index.lock")
	info, err := os.Stat(lock)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if time.Since(info.ModTime()) > staleLockThreshold {
		slog.Warn("git: removing stale index.lock", "path", lock)
		return os.Remove(lock)
	}
	return nil
}

// Read-методы — pure pass-through.

// GetByID проксирует вызов в inner.
func (d *GitDecorator) GetByID(ctx context.Context, id core.PostID) (*core.Post, error) {
	return d.inner.GetByID(ctx, id)
}

// GetBySlug проксирует вызов в inner.
func (d *GitDecorator) GetBySlug(ctx context.Context, slug string) (*core.Post, error) {
	return d.inner.GetBySlug(ctx, slug)
}

// List проксирует вызов в inner.
func (d *GitDecorator) List(ctx context.Context, filter core.PostFilter) ([]*core.Post, error) {
	return d.inner.List(ctx, filter)
}

// Create вызывает inner.Create и при успехе делает git commit (+ push).
func (d *GitDecorator) Create(ctx context.Context, post *core.Post) error {
	if err := d.inner.Create(ctx, post); err != nil {
		return err
	}
	d.afterMutate(ctx, "create", post)
	return nil
}

// Update вызывает inner.Update и при успехе делает git commit (+ push).
func (d *GitDecorator) Update(ctx context.Context, post *core.Post) error {
	if err := d.inner.Update(ctx, post); err != nil {
		return err
	}
	d.afterMutate(ctx, "update", post)
	return nil
}

// Delete вызывает inner.Delete и при успехе делает git commit (+ push).
func (d *GitDecorator) Delete(ctx context.Context, id core.PostID) error {
	// Получаем пост ДО удаления — нужна информация для commit message и пути.
	var post *core.Post
	if p, err := d.inner.GetByID(ctx, id); err == nil {
		post = p
	}
	if err := d.inner.Delete(ctx, id); err != nil {
		return err
	}
	if post == nil {
		// inner.Delete прошёл, но мы не смогли получить метаданные.
		// Делаем минимальный commit "delete" со stub-постом.
		post = &core.Post{Slug: id.String(), Status: core.PostStatus("")}
	}
	d.afterMutate(ctx, "delete", post)
	return nil
}

// ImportPosts проксирует вызов в MigratableRepository inner или, если inner
// его не поддерживает (FS), фолбэчится на per-post Create. В обоих случаях
// после успеха создаётся ОДИН batch-commit.
func (d *GitDecorator) ImportPosts(ctx context.Context, posts []*core.Post) error {
	if mig, ok := d.inner.(core.MigratableRepository); ok {
		if err := mig.ImportPosts(ctx, posts); err != nil {
			return err
		}
	} else {
		for _, p := range posts {
			if err := d.inner.Create(ctx, p); err != nil {
				return err
			}
		}
	}
	if d.detached || len(posts) == 0 {
		return nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	wt, err := d.repo.Worktree()
	if err != nil {
		slog.Warn("git: import worktree failed", "err", err)
		return nil
	}
	for _, p := range posts {
		if _, err := wt.Add(d.relativePath(p)); err != nil {
			slog.Warn("git: add failed during import", "err", err, "slug", p.Slug)
		}
	}
	msg := fmt.Sprintf("chore: import %d posts", len(posts))
	if _, err := wt.Commit(msg, &git.CommitOptions{Author: gitAuthor()}); err != nil {
		slog.Warn("git: import commit failed", "err", err)
		return nil
	}
	if d.cfg.AutoPush {
		if err := d.pushChanges(ctx); err != nil {
			slog.Warn("git: import push failed", "err", err)
		}
	}
	return nil
}

// Count проксирует в MigratableRepository inner или, если inner
// его не поддерживает (FS), считает .md-файлы во всех tenant-подкаталогах.
func (d *GitDecorator) Count(ctx context.Context) (int64, error) {
	if mig, ok := d.inner.(core.MigratableRepository); ok {
		return mig.Count(ctx)
	}
	var count int64
	err := filepath.WalkDir(d.postsDir, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == ".git" {
			return filepath.SkipDir
		}
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			count++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

// Close закрывает inner-ресурсы, если он реализует io.Closer.
func (d *GitDecorator) Close() error {
	if c, ok := d.inner.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// afterMutate инкапсулирует сериализованный git commit + push после успешной мутации.
func (d *GitDecorator) afterMutate(ctx context.Context, op string, post *core.Post) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.detached {
		slog.Warn("git: skipping commit (detached HEAD)", "op", op, "slug", post.Slug)
		return
	}

	if err := d.commitChanges(op, post); err != nil {
		slog.Warn("git: commit failed", "err", err, "op", op, "slug", post.Slug)
		return
	}
	if d.cfg.AutoPush {
		if err := d.pushChanges(ctx); err != nil {
			slog.Warn("git: push failed", "err", err, "remote", maskURL(d.cfg.Remote))
		}
	}
}

// commitChanges стейджит файлы и делает один commit. Должна вызываться под d.mu.
func (d *GitDecorator) commitChanges(op string, post *core.Post) error {
	wt, err := d.repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}
	rel := d.relativePath(post)
	if op == "delete" {
		if _, err := wt.Remove(rel); err != nil {
			// fallback: stage all changes.
			if _, err2 := wt.Add("."); err2 != nil {
				return fmt.Errorf("stage delete: %w", err2)
			}
		}
	} else {
		if _, err := wt.Add(rel); err != nil {
			return fmt.Errorf("add %s: %w", rel, err)
		}
	}
	msg := renderMessage(d.template, op, post)
	if _, err := wt.Commit(msg, &git.CommitOptions{Author: gitAuthor()}); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// pushChanges пушит на remote с timeout=30s.
func (d *GitDecorator) pushChanges(ctx context.Context) error {
	pctx, cancel := context.WithTimeout(ctx, pushTimeout)
	defer cancel()
	auth := resolveAuth(d.cfg.Remote)
	err := d.repo.PushContext(pctx, &git.PushOptions{
		RemoteName: "origin",
		Auth:       auth,
	})
	if err == nil || errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil
	}
	return err
}

// relativePath возвращает путь к посту относительно postsDir.
func (d *GitDecorator) relativePath(post *core.Post) string {
	return filepath.Join(post.TenantShortID(), post.Slug+".md")
}

// resolveAuth выбирает auth-метод по схеме remote URL.
func resolveAuth(remote string) transport.AuthMethod {
	if strings.HasPrefix(remote, "https://") || strings.HasPrefix(remote, "http://") {
		if token := os.Getenv("GIT_HTTPS_TOKEN"); token != "" {
			return &githttp.BasicAuth{Username: "token", Password: token}
		}
		return nil
	}
	if strings.HasPrefix(remote, "git@") || strings.HasPrefix(remote, "ssh://") {
		if a, err := gitssh.NewSSHAgentAuth("git"); err == nil {
			return a
		}
		return nil
	}
	return nil
}

// maskURL скрывает userinfo в URL для логов.
func maskURL(u string) string {
	if i := strings.Index(u, "@"); i > 0 {
		if j := strings.Index(u, "://"); j >= 0 && j < i {
			return u[:j+3] + "***@" + u[i+1:]
		}
	}
	return u
}
