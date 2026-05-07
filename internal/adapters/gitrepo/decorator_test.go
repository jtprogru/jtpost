package gitrepo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/google/uuid"

	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/adapters/fsrepo"
	"github.com/jtprogru/jtpost/internal/core"
)

// fixedTenantID — стабильный tenant для тестов.
//
//nolint:gochecknoglobals // тестовый константный fixture
var fixedTenantID = uuid.MustParse("11111111-1111-1111-1111-111111111111")

func testCtx() context.Context {
	return core.WithTenant(context.Background(), fixedTenantID)
}

func mkPost(slug string) *core.Post {
	now := time.Now().UTC()
	return &core.Post{
		ID:        core.PostID(uuid.New()),
		TenantID:  fixedTenantID,
		AuthorID:  uuid.New(),
		Title:     "T-" + slug,
		Slug:      slug,
		Status:    core.StatusDraft,
		CreatedAt: now,
		UpdatedAt: now,
		Revision:  1,
	}
}

// newDecorator создаёт fsrepo + GitDecorator в TempDir.
func newDecorator(t *testing.T, cfg config.GitStorageConfig) (*GitDecorator, string) {
	t.Helper()
	dir := t.TempDir()
	inner, err := fsrepo.NewFileSystemRepository(dir)
	if err != nil {
		t.Fatalf("fsrepo: %v", err)
	}
	if cfg.Branch == "" {
		cfg.Branch = "main"
	}
	d, err := NewGitDecorator(inner, dir, cfg)
	if err != nil {
		t.Fatalf("decorator: %v", err)
	}
	return d, dir
}

func countCommits(t *testing.T, dir string) int {
	t.Helper()
	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	head, err := repo.Head()
	if err != nil {
		// Возможно, нет коммитов вовсе.
		return 0
	}
	iter, err := repo.Log(&git.LogOptions{From: head.Hash()})
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	n := 0
	_ = iter.ForEach(func(_ *object.Commit) error {
		n++
		return nil
	})
	iter.Close()
	return n
}

func latestCommitMsg(t *testing.T, dir string) string {
	t.Helper()
	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	head, err := repo.Head()
	if err != nil {
		t.Fatalf("head: %v", err)
	}
	c, err := repo.CommitObject(head.Hash())
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	return c.Message
}

func TestNewGitDecorator_AutoInit(t *testing.T) {
	_, dir := newDecorator(t, config.GitStorageConfig{Enabled: true, Branch: "main"})
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Fatalf(".git not created: %v", err)
	}
	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	ref, err := repo.Reference(plumbing.HEAD, false)
	if err != nil {
		t.Fatalf("HEAD: %v", err)
	}
	if got := ref.Target().String(); got != "refs/heads/main" {
		t.Errorf("HEAD target = %q, want refs/heads/main", got)
	}
}

func TestNewGitDecorator_ExistingRepo(t *testing.T) {
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Storer.SetReference(plumbing.NewSymbolicReference(
		plumbing.HEAD, plumbing.NewBranchReferenceName("feature/x"),
	)); err != nil {
		t.Fatal(err)
	}

	inner, _ := fsrepo.NewFileSystemRepository(dir)
	_, err = NewGitDecorator(inner, dir, config.GitStorageConfig{Branch: "main"})
	if err != nil {
		t.Fatalf("decorator: %v", err)
	}
	repo2, _ := git.PlainOpen(dir)
	ref, _ := repo2.Reference(plumbing.HEAD, false)
	if got := ref.Target().String(); got != "refs/heads/feature/x" {
		t.Errorf("branch switched: %q", got)
	}
}

func TestNewGitDecorator_DetachedHEAD(t *testing.T) {
	d, dir := newDecorator(t, config.GitStorageConfig{Branch: "main"})
	// Сделаем коммит, чтобы получить хэш.
	post := mkPost("a")
	if err := d.Create(testCtx(), post); err != nil {
		t.Fatal(err)
	}
	repo, _ := git.PlainOpen(dir)
	head, _ := repo.Head()
	hash := head.Hash()

	// Поставим HEAD напрямую на hash (detached).
	if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.HEAD, hash)); err != nil {
		t.Fatal(err)
	}

	inner, _ := fsrepo.NewFileSystemRepository(dir)
	d2, err := NewGitDecorator(inner, dir, config.GitStorageConfig{Branch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if !d2.detached {
		t.Errorf("expected detached=true")
	}
	commitsBefore := countCommits(t, dir)
	if err := d2.Create(testCtx(), mkPost("b")); err != nil {
		t.Fatalf("create: %v", err)
	}
	commitsAfter := countCommits(t, dir)
	if commitsBefore != commitsAfter {
		t.Errorf("detached should not commit: before=%d after=%d", commitsBefore, commitsAfter)
	}
}

func TestNewGitDecorator_StaleLock(t *testing.T) {
	_, dir := newDecorator(t, config.GitStorageConfig{Branch: "main"})
	lock := filepath.Join(dir, ".git", "index.lock")
	if err := os.WriteFile(lock, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Minute)
	if err := os.Chtimes(lock, old, old); err != nil {
		t.Fatal(err)
	}
	// Повторный Open должен удалить stale lock.
	inner, _ := fsrepo.NewFileSystemRepository(dir)
	if _, err := NewGitDecorator(inner, dir, config.GitStorageConfig{Branch: "main"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(lock); !os.IsNotExist(err) {
		t.Errorf("stale lock not removed: %v", err)
	}
}

func TestNewGitDecorator_FreshLock(t *testing.T) {
	_, dir := newDecorator(t, config.GitStorageConfig{Branch: "main"})
	lock := filepath.Join(dir, ".git", "index.lock")
	if err := os.WriteFile(lock, []byte("fresh"), 0o644); err != nil {
		t.Fatal(err)
	}
	inner, _ := fsrepo.NewFileSystemRepository(dir)
	if _, err := NewGitDecorator(inner, dir, config.GitStorageConfig{Branch: "main"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(lock); err != nil {
		t.Errorf("fresh lock should remain: %v", err)
	}
}

func TestNewGitDecorator_InvalidTemplate(t *testing.T) {
	dir := t.TempDir()
	inner, _ := fsrepo.NewFileSystemRepository(dir)
	_, err := NewGitDecorator(inner, dir, config.GitStorageConfig{
		Branch:         "main",
		CommitTemplate: "{{.Slug",
	})
	if !errors.Is(err, core.ErrConfigInvalid) {
		t.Errorf("want ErrConfigInvalid, got %v", err)
	}
}

func TestGitDecorator_Create_AddsCommit(t *testing.T) {
	d, dir := newDecorator(t, config.GitStorageConfig{Branch: "main"})
	post := mkPost("hello")
	if err := d.Create(testCtx(), post); err != nil {
		t.Fatalf("create: %v", err)
	}
	if got := countCommits(t, dir); got != 1 {
		t.Errorf("expected 1 commit, got %d", got)
	}
	msg := latestCommitMsg(t, dir)
	if msg != "chore: create post hello" {
		t.Errorf("msg = %q", msg)
	}
}

func TestGitDecorator_Update_AddsCommit(t *testing.T) {
	d, dir := newDecorator(t, config.GitStorageConfig{Branch: "main"})
	post := mkPost("upd")
	if err := d.Create(testCtx(), post); err != nil {
		t.Fatal(err)
	}
	post.Title = "Updated"
	post.Revision = 2
	post.UpdatedAt = time.Now().UTC()
	if err := d.Update(testCtx(), post); err != nil {
		t.Fatalf("update: %v", err)
	}
	if got := countCommits(t, dir); got != 2 {
		t.Errorf("expected 2 commits, got %d", got)
	}
	if got := latestCommitMsg(t, dir); got != "chore: update post upd" {
		t.Errorf("msg = %q", got)
	}
}

func TestGitDecorator_Delete_AddsCommit(t *testing.T) {
	d, dir := newDecorator(t, config.GitStorageConfig{Branch: "main"})
	post := mkPost("del")
	if err := d.Create(testCtx(), post); err != nil {
		t.Fatal(err)
	}
	if err := d.Delete(testCtx(), post.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got := countCommits(t, dir); got != 2 {
		t.Errorf("expected 2 commits, got %d", got)
	}
	if got := latestCommitMsg(t, dir); got != "chore: delete post del" {
		t.Errorf("msg = %q", got)
	}
}

// --- mock inner для test-сценариев с failing inner ---

type failingInner struct {
	err error
}

func (m *failingInner) GetByID(_ context.Context, _ core.PostID) (*core.Post, error) {
	return nil, m.err
}
func (m *failingInner) GetBySlug(_ context.Context, _ string) (*core.Post, error) {
	return nil, m.err
}
func (m *failingInner) List(_ context.Context, _ core.PostFilter) ([]*core.Post, error) {
	return nil, m.err
}
func (m *failingInner) Create(_ context.Context, _ *core.Post) error { return m.err }
func (m *failingInner) Update(_ context.Context, _ *core.Post) error { return m.err }
func (m *failingInner) Delete(_ context.Context, _ core.PostID) error {
	return m.err
}

func TestGitDecorator_Create_InnerFail_NoCommit(t *testing.T) {
	dir := t.TempDir()
	inner := &failingInner{err: core.ErrValidation}
	d, err := NewGitDecorator(inner, dir, config.GitStorageConfig{Branch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	before := countCommits(t, dir)
	err = d.Create(testCtx(), mkPost("x"))
	if !errors.Is(err, core.ErrValidation) {
		t.Errorf("want ErrValidation, got %v", err)
	}
	after := countCommits(t, dir)
	if before != after {
		t.Errorf("no commit expected on inner failure: before=%d after=%d", before, after)
	}
}

// --- mock inner с заранее заготовленными постами для read pass-through ---

type stubInner struct {
	getByID    *core.Post
	getBySlug  *core.Post
	listResult []*core.Post
	calls      map[string]int
}

func newStubInner() *stubInner {
	return &stubInner{calls: map[string]int{}}
}

func (s *stubInner) GetByID(_ context.Context, _ core.PostID) (*core.Post, error) {
	s.calls["GetByID"]++
	return s.getByID, nil
}
func (s *stubInner) GetBySlug(_ context.Context, _ string) (*core.Post, error) {
	s.calls["GetBySlug"]++
	return s.getBySlug, nil
}
func (s *stubInner) List(_ context.Context, _ core.PostFilter) ([]*core.Post, error) {
	s.calls["List"]++
	return s.listResult, nil
}
func (s *stubInner) Create(_ context.Context, _ *core.Post) error { return nil }
func (s *stubInner) Update(_ context.Context, _ *core.Post) error { return nil }
func (s *stubInner) Delete(_ context.Context, _ core.PostID) error {
	return nil
}

func TestGitDecorator_Read_PassThrough(t *testing.T) {
	dir := t.TempDir()
	stub := newStubInner()
	stub.getByID = mkPost("by-id")
	stub.getBySlug = mkPost("by-slug")
	stub.listResult = []*core.Post{mkPost("a"), mkPost("b")}

	d, err := NewGitDecorator(stub, dir, config.GitStorageConfig{Branch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if p, _ := d.GetByID(testCtx(), stub.getByID.ID); p != stub.getByID {
		t.Errorf("GetByID mismatch")
	}
	if p, _ := d.GetBySlug(testCtx(), "by-slug"); p != stub.getBySlug {
		t.Errorf("GetBySlug mismatch")
	}
	if list, _ := d.List(testCtx(), core.PostFilter{TenantID: fixedTenantID}); len(list) != 2 {
		t.Errorf("List len = %d", len(list))
	}
	if got := countCommits(t, dir); got != 0 {
		t.Errorf("read should not commit: got %d commits", got)
	}
}

func TestGitDecorator_Detached_NoCommit(t *testing.T) {
	d, dir := newDecorator(t, config.GitStorageConfig{Branch: "main"})
	d.detached = true
	if err := d.Create(testCtx(), mkPost("nope")); err != nil {
		t.Fatal(err)
	}
	if got := countCommits(t, dir); got != 0 {
		t.Errorf("detached: expected 0 commits, got %d", got)
	}
}

// --- ImportPosts batch commit (через MigratableRepository inner) ---

type migratableStub struct {
	*fsrepo.FileSystemPostRepository
}

func (m *migratableStub) ImportPosts(ctx context.Context, posts []*core.Post) error {
	for _, p := range posts {
		if err := m.Create(ctx, p); err != nil {
			return err
		}
	}
	return nil
}

func (m *migratableStub) Count(_ context.Context) (int64, error) {
	return 0, nil
}

func TestGitDecorator_ImportPosts_BatchCommit(t *testing.T) {
	dir := t.TempDir()
	inner, _ := fsrepo.NewFileSystemRepository(dir)
	mig := &migratableStub{FileSystemPostRepository: inner}
	d, err := NewGitDecorator(mig, dir, config.GitStorageConfig{Branch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	posts := []*core.Post{mkPost("a"), mkPost("b"), mkPost("c")}
	if err := d.ImportPosts(testCtx(), posts); err != nil {
		t.Fatalf("import: %v", err)
	}
	if got := countCommits(t, dir); got != 1 {
		t.Errorf("expected 1 batch commit, got %d", got)
	}
}

func TestGitDecorator_Concurrent_NoLockCollision(t *testing.T) {
	d, dir := newDecorator(t, config.GitStorageConfig{Branch: "main"})
	const N = 10
	var wg sync.WaitGroup
	wg.Add(N)
	errs := make(chan error, N)
	for range N {
		go func() {
			defer wg.Done()
			post := mkPost("p")
			post.Slug = "concurrent-" + post.ID.String()[:8]
			if err := d.Create(testCtx(), post); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Errorf("create failed: %v", e)
	}
	if got := countCommits(t, dir); got != N {
		t.Errorf("expected %d commits, got %d", N, got)
	}
}
