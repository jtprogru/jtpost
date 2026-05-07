package gitrepo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/go-git/go-git/v5"
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
