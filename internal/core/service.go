package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PostService сервис для управления постами.
type PostService struct {
	repo  PostRepository
	clock Clock
}

// NewPostService создаёт новый PostService.
func NewPostService(repo PostRepository, clock Clock) *PostService {
	return &PostService{
		repo:  repo,
		clock: clock,
	}
}

// CreatePostInput входные данные для создания поста.
type CreatePostInput struct {
	TenantID uuid.UUID
	AuthorID uuid.UUID
	Title    string
	Tags     []string
	Slug     string  // опционально, если не указан — сгенерируем
	Excerpt  *string // опционально
}

// CreatePost создаёт новый пост.
func (s *PostService) CreatePost(ctx context.Context, in CreatePostInput) (*Post, error) {
	if in.TenantID == uuid.Nil {
		return nil, fmt.Errorf("%w: tenant_id required", ErrValidation)
	}
	if in.AuthorID == uuid.Nil {
		return nil, fmt.Errorf("%w: author_id required", ErrValidation)
	}
	if in.Title == "" {
		return nil, ErrEmptyTitle
	}

	slug := in.Slug
	if slug == "" {
		slug = s.generateSlug(in.Title)
	}

	now := s.clock.Now()
	post := &Post{
		ID:        GeneratePostID(slug, now),
		TenantID:  in.TenantID,
		AuthorID:  in.AuthorID,
		Title:     in.Title,
		Slug:      slug,
		Status:    StatusIdea,
		CreatedAt: now,
		UpdatedAt: now,
		Revision:  1,
		Tags:      in.Tags,
		Excerpt:   in.Excerpt,
		Content:   "",
		External:  ExternalLinks{},
	}

	if err := s.repo.Create(ctx, post); err != nil {
		return nil, err
	}
	return post, nil
}

// GetByID возвращает пост по идентификатору.
func (s *PostService) GetByID(ctx context.Context, id PostID) (*Post, error) {
	return s.repo.GetByID(ctx, id)
}

// GetBySlug возвращает пост по slug.
func (s *PostService) GetBySlug(ctx context.Context, slug string) (*Post, error) {
	return s.repo.GetBySlug(ctx, slug)
}

// CreatePostWithContent создаёт пост с готовым контентом (для импорта).
//
// Поля TenantID/AuthorID должны быть установлены вызывающим кодом.
// CreatedAt/UpdatedAt/Revision устанавливаются здесь, если не заданы.
func (s *PostService) CreatePostWithContent(ctx context.Context, post *Post) error {
	if post.TenantID == uuid.Nil {
		return fmt.Errorf("%w: tenant_id required", ErrValidation)
	}
	if post.AuthorID == uuid.Nil {
		return fmt.Errorf("%w: author_id required", ErrValidation)
	}
	if post.Title == "" {
		return ErrEmptyTitle
	}
	if post.Slug == "" {
		return ErrEmptySlug
	}
	now := s.clock.Now()
	if post.ID == (PostID{}) {
		post.ID = GeneratePostID(post.Slug, now)
	}
	if post.CreatedAt.IsZero() {
		post.CreatedAt = now
	}
	if post.UpdatedAt.IsZero() {
		post.UpdatedAt = now
	}
	if post.Revision == 0 {
		post.Revision = 1
	}
	return s.repo.Create(ctx, post)
}

// ListPosts возвращает список постов с фильтрами.
func (s *PostService) ListPosts(ctx context.Context, filter PostFilter) ([]*Post, error) {
	if filter.SortBy != "" && !IsValidSortKey(filter.SortBy) {
		return nil, fmt.Errorf("%w: invalid sort key %q", ErrValidation, filter.SortBy)
	}
	return s.repo.List(ctx, filter)
}

// UpdateStatus обновляет статус поста с проверкой допустимости перехода.
func (s *PostService) UpdateStatus(ctx context.Context, id PostID, newStatus PostStatus) (*Post, error) {
	post, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if !IsTransitionAllowed(post.Status, newStatus) {
		return nil, fmt.Errorf("%w: cannot transition from %s to %s", ErrInvalidTransition, post.Status, newStatus)
	}

	post.Status = newStatus
	if newStatus == StatusPublished && post.PublishedAt == nil {
		now := s.clock.Now()
		post.PublishedAt = &now
	}
	return s.updateInternal(ctx, post)
}

// UpdatePost обновляет пост с проверкой tenant immutability и инкрементом Revision.
func (s *PostService) UpdatePost(ctx context.Context, post *Post) error {
	if post.Title == "" {
		return ErrEmptyTitle
	}
	if post.Slug == "" {
		return ErrEmptySlug
	}

	existing, err := s.repo.GetByID(ctx, post.ID)
	if err != nil {
		return err
	}
	if existing.TenantID != post.TenantID {
		return ErrTenantMismatch
	}

	// CreatedAt не должен изменяться через UpdatePost.
	post.CreatedAt = existing.CreatedAt
	post.AuthorID = existing.AuthorID
	post.UpdatedAt = s.clock.Now()
	post.Revision = existing.Revision + 1

	return s.repo.Update(ctx, post)
}

// Archive переводит пост в статус archived. Допустимо только из published или
// failed (см. allowedTransitions).
func (s *PostService) Archive(ctx context.Context, id PostID) (*Post, error) {
	post, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !IsTransitionAllowed(post.Status, StatusArchived) {
		return nil, fmt.Errorf("%w: cannot archive from %s", ErrInvalidTransition, post.Status)
	}
	post.Status = StatusArchived
	return s.updateInternal(ctx, post)
}

// MarkFailed переводит пост в статус failed и добавляет запись в PublishHistory.
// Допустимо только из scheduled.
func (s *PostService) MarkFailed(ctx context.Context, id PostID, errMsg string) (*Post, error) {
	post, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !IsTransitionAllowed(post.Status, StatusFailed) {
		return nil, fmt.Errorf("%w: cannot mark failed from %s", ErrInvalidTransition, post.Status)
	}
	post.Status = StatusFailed
	post.PublishHistory = append(post.PublishHistory, PublishAttempt{
		ID:           uuid.New(),
		At:           s.clock.Now(),
		Target:       "telegram",
		Status:       "failed",
		Error:        errMsg,
		RetryAttempt: len(post.PublishHistory) + 1,
	})
	return s.updateInternal(ctx, post)
}

// AppendPublishAttempt добавляет запись в PublishHistory без смены статуса.
// Используется publisher'ом для логирования попыток.
func (s *PostService) AppendPublishAttempt(ctx context.Context, id PostID, attempt PublishAttempt) error {
	post, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	post.PublishHistory = append(post.PublishHistory, attempt)
	_, err = s.updateInternal(ctx, post)
	return err
}

// DeletePost удаляет пост.
func (s *PostService) DeletePost(ctx context.Context, id PostID) error {
	return s.repo.Delete(ctx, id)
}

// PostStats статистика по постам.
type PostStats struct {
	Total    int                `json:"total"`
	ByStatus map[PostStatus]int `json:"by_status"`
	ByTag    map[string]int     `json:"by_tag"`
}

// GetStats возвращает статистику по постам в заданном tenant scope.
func (s *PostService) GetStats(ctx context.Context, tenantID uuid.UUID) (*PostStats, error) {
	posts, err := s.repo.List(ctx, PostFilter{TenantID: tenantID})
	if err != nil {
		return nil, err
	}

	stats := &PostStats{
		Total:    len(posts),
		ByStatus: make(map[PostStatus]int),
		ByTag:    make(map[string]int),
	}

	for _, post := range posts {
		stats.ByStatus[post.Status]++
		for _, tag := range post.Tags {
			stats.ByTag[tag]++
		}
	}
	return stats, nil
}

// GetNextPost возвращает следующий пост для работы в заданном tenant scope.
//
// Приоритеты:
//  1. Посты с дедлайном (просроченный — выше).
//  2. Посты со scheduled_at.
//  3. По статусу (ready > draft > idea).
func (s *PostService) GetNextPost(ctx context.Context, tenantID uuid.UUID) (*Post, error) {
	posts, err := s.repo.List(ctx, PostFilter{TenantID: tenantID})
	if err != nil {
		return nil, err
	}

	var candidates []*Post
	for _, post := range posts {
		switch post.Status {
		case StatusPublished, StatusScheduled, StatusArchived, StatusFailed:
			continue
		}
		candidates = append(candidates, post)
	}
	if len(candidates) == 0 {
		return nil, nil //nolint:nilnil
	}

	now := s.clock.Now()
	sortPostsByPriority(candidates, now)
	return candidates[0], nil
}

// sortPostsByPriority сортирует посты по приоритету (на месте).
func sortPostsByPriority(posts []*Post, now time.Time) {
	//nolint:intrange
	for i := 0; i < len(posts)-1; i++ {
		for j := 0; j < len(posts)-i-1; j++ {
			if postPriority(posts[j], now) > postPriority(posts[j+1], now) {
				posts[j], posts[j+1] = posts[j+1], posts[j]
			}
		}
	}
}

// postPriority возвращает числовой приоритет поста (меньше = выше приоритет).
func postPriority(post *Post, now time.Time) int {
	if post.Deadline != nil {
		if post.Deadline.Before(now) {
			return -1000 + int(now.Sub(*post.Deadline).Hours())
		}
		return int(post.Deadline.Sub(now).Hours())
	}
	if post.ScheduledAt != nil {
		if post.ScheduledAt.Before(now) {
			return -500 + int(now.Sub(*post.ScheduledAt).Hours())
		}
		return 10000 + int(post.ScheduledAt.Sub(now).Hours())
	}
	switch post.Status {
	case StatusReady:
		return 20000
	case StatusDraft:
		return 30000
	case StatusIdea:
		return 40000
	default:
		return 50000
	}
}

// PublishPostInput входные данные для публикации поста.
type PublishPostInput struct{}

// PublishPost публикует пост в Telegram.
func (s *PostService) PublishPost(ctx context.Context, id PostID, _ PublishPostInput, publisher Publisher) (*Post, error) {
	post, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if post.Status == StatusPublished {
		return nil, fmt.Errorf("%w: пост уже опубликован", ErrInvalidStatus)
	}
	if post.Content == "" {
		return nil, fmt.Errorf("%w: пустой контент поста", ErrValidation)
	}

	updatedPost, err := publisher.Publish(ctx, post)
	if err != nil {
		return nil, fmt.Errorf("ошибка публикации: %w", err)
	}
	post = updatedPost
	post.Status = StatusPublished
	now := s.clock.Now()
	post.PublishedAt = &now

	if _, err := s.updateInternal(ctx, post); err != nil {
		return nil, fmt.Errorf("ошибка обновления поста: %w", err)
	}
	return post, nil
}

// updateInternal — внутренний путь обновления, который инкрементирует Revision и
// UpdatedAt, не выполняя повторной проверки tenant. Используется методами
// сервиса, которые сами загрузили пост из репозитория.
func (s *PostService) updateInternal(ctx context.Context, post *Post) (*Post, error) {
	post.UpdatedAt = s.clock.Now()
	post.Revision++
	if err := s.repo.Update(ctx, post); err != nil {
		return nil, err
	}
	return post, nil
}

// generateSlug генерирует slug из заголовка.
func (s *PostService) generateSlug(title string) string {
	return generateSlug(title)
}

// errIsValidation возвращает true, если err оборачивает ErrValidation.
//
//nolint:unused // экспортированная утилита для тестов в одной горутине.
func errIsValidation(err error) bool {
	return errors.Is(err, ErrValidation)
}
