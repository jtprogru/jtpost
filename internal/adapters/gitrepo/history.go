package gitrepo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/jtprogru/jtpost/internal/core"
)

const historyDefaultLimit = 50

// History возвращает git-log для файла поста. Лимит ≤ historyDefaultLimit (50)
// если limit ≤ 0 или > 50. Если репозиторий пустой (no commits) — возвращает
// nil без ошибки.
func (d *GitDecorator) History(ctx context.Context, post *core.Post, limit int) ([]core.HistoryEntry, error) {
	if post == nil {
		return nil, errors.New("post is nil")
	}
	if limit <= 0 || limit > historyDefaultLimit {
		limit = historyDefaultLimit
	}
	relPath := d.relativePath(post)

	d.mu.Lock()
	defer d.mu.Unlock()

	logIter, err := d.repo.Log(&git.LogOptions{FileName: &relPath})
	if err != nil {
		// Пустой репо без HEAD → trivially no history.
		if errors.Is(err, git.ErrEmptyCommit) || strings.Contains(err.Error(), "reference not found") {
			return nil, nil
		}
		return nil, fmt.Errorf("git log: %w", err)
	}
	defer logIter.Close()

	out := make([]core.HistoryEntry, 0, limit)
	err = logIter.ForEach(func(c *object.Commit) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if len(out) >= limit {
			return io.EOF
		}
		hash := c.Hash.String()
		short := hash
		if len(short) > 8 {
			short = short[:8]
		}
		msg := strings.SplitN(strings.TrimSpace(c.Message), "\n", 2)[0]
		out = append(out, core.HistoryEntry{
			Hash:      hash,
			ShortHash: short,
			Author:    c.Committer.Name,
			Email:     c.Committer.Email,
			Message:   msg,
			At:        c.Committer.When,
		})
		return nil
	})
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("git log iter: %w", err)
	}
	return out, nil
}

// FileAtCommit возвращает содержимое файла поста на указанном commit hash.
// Принимает полный или короткий hash (≥4 символов). ErrNotFound — если файла
// не было в коммите или коммит не найден.
func (d *GitDecorator) FileAtCommit(_ context.Context, post *core.Post, commitHash string) ([]byte, error) {
	if post == nil {
		return nil, errors.New("post is nil")
	}
	commitHash = strings.TrimSpace(commitHash)
	if len(commitHash) < 4 {
		return nil, fmt.Errorf("%w: invalid commit hash %q", core.ErrValidation, commitHash)
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	hash, err := d.resolveHash(commitHash)
	if err != nil {
		return nil, err
	}
	commit, err := d.repo.CommitObject(hash)
	if err != nil {
		return nil, fmt.Errorf("commit %s: %w", hash, err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("tree: %w", err)
	}
	relPath := d.relativePath(post)
	file, err := tree.File(relPath)
	if err != nil {
		if errors.Is(err, object.ErrFileNotFound) {
			return nil, core.ErrNotFound
		}
		return nil, fmt.Errorf("file %s: %w", relPath, err)
	}
	reader, err := file.Reader()
	if err != nil {
		return nil, fmt.Errorf("file reader: %w", err)
	}
	defer func() { _ = reader.Close() }()
	return io.ReadAll(reader)
}

// resolveHash принимает полный или короткий hash и возвращает полный.
// go-git до v5.x не имеет встроенного prefix-resolver — обходим через
// CommitObjects iter с префикс-проверкой.
func (d *GitDecorator) resolveHash(prefix string) (plumbing.Hash, error) {
	if len(prefix) == 40 {
		return plumbing.NewHash(prefix), nil
	}
	iter, err := d.repo.CommitObjects()
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("commit iter: %w", err)
	}
	defer iter.Close()
	prefix = strings.ToLower(prefix)
	var match plumbing.Hash
	found := false
	err = iter.ForEach(func(c *object.Commit) error {
		if strings.HasPrefix(c.Hash.String(), prefix) {
			if found {
				return fmt.Errorf("%w: ambiguous prefix %q", core.ErrValidation, prefix)
			}
			match = c.Hash
			found = true
		}
		return nil
	})
	if err != nil {
		return plumbing.ZeroHash, err
	}
	if !found {
		return plumbing.ZeroHash, core.ErrNotFound
	}
	return match, nil
}
