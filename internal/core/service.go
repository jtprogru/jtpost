package core

import (
	"context"
	"fmt"
	"time"
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
	Title     string
	Platforms []Platform
	Tags      []string
	Slug      string // опционально, если не указан — сгенерируем
}

// CreatePost создаёт новый пост.
func (s *PostService) CreatePost(ctx context.Context, in CreatePostInput) (*Post, error) {
	if in.Title == "" {
		return nil, ErrEmptyTitle
	}

	slug := in.Slug
	if slug == "" {
		slug = s.generateSlug(in.Title)
	}

	now := s.clock.Now()
	post := &Post{
		ID:        PostID(fmt.Sprintf("%d-%s", now.UnixNano(), slug)),
		Title:     in.Title,
		Slug:      slug,
		Status:    StatusIdea,
		Platforms: in.Platforms,
		Tags:      in.Tags,
		Content:   "",
		External:  ExternalLinks{},
	}

	if len(post.Platforms) == 0 {
		post.Platforms = []Platform{PlatformTelegram}
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
func (s *PostService) CreatePostWithContent(ctx context.Context, post *Post) error {
	if post.Title == "" {
		return ErrEmptyTitle
	}
	if post.Slug == "" {
		return ErrEmptySlug
	}
	if post.ID == "" {
		now := s.clock.Now()
		post.ID = PostID(fmt.Sprintf("%d-%s", now.UnixNano(), post.Slug))
	}
	return s.repo.Create(ctx, post)
}

// ListPosts возвращает список постов с фильтрами.
func (s *PostService) ListPosts(ctx context.Context, filter PostFilter) ([]*Post, error) {
	return s.repo.List(ctx, filter)
}

// UpdateStatus обновляет статус поста.
func (s *PostService) UpdateStatus(ctx context.Context, id PostID, newStatus PostStatus) (*Post, error) {
	post, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if !IsStatusTransitionValid(post.Status, newStatus) {
		return nil, fmt.Errorf("%w: cannot transition from %s to %s", ErrInvalidStatus, post.Status, newStatus)
	}

	post.Status = newStatus

	if newStatus == StatusPublished {
		now := s.clock.Now()
		post.PublishedAt = &now
	}

	if err := s.repo.Update(ctx, post); err != nil {
		return nil, err
	}

	return post, nil
}

// UpdatePost обновляет пост.
func (s *PostService) UpdatePost(ctx context.Context, post *Post) error {
	if post.Title == "" {
		return ErrEmptyTitle
	}
	if post.Slug == "" {
		return ErrEmptySlug
	}
	return s.repo.Update(ctx, post)
}

// DeletePost удаляет пост.
func (s *PostService) DeletePost(ctx context.Context, id PostID) error {
	return s.repo.Delete(ctx, id)
}

// PostStats статистика по постам.
type PostStats struct {
	Total      int                     `json:"total"`
	ByStatus   map[PostStatus]int      `json:"by_status"`
	ByPlatform map[Platform]int        `json:"by_platform"`
	ByTag      map[string]int          `json:"by_tag"`
}

// GetStats возвращает статистику по постам.
func (s *PostService) GetStats(ctx context.Context) (*PostStats, error) {
	posts, err := s.repo.List(ctx, PostFilter{})
	if err != nil {
		return nil, err
	}

	stats := &PostStats{
		Total:      len(posts),
		ByStatus:   make(map[PostStatus]int),
		ByPlatform: make(map[Platform]int),
		ByTag:      make(map[string]int),
	}

	for _, post := range posts {
		stats.ByStatus[post.Status]++

		for _, platform := range post.Platforms {
			stats.ByPlatform[platform]++
		}

		for _, tag := range post.Tags {
			stats.ByTag[tag]++
		}
	}

	return stats, nil
}

// GetNextPost возвращает следующий пост для работы на основе приоритетов:
// 1. Посты с дедлайном (ближайший дедлайн — выше приоритет).
// 2. Посты со scheduled_at (ближайшая публикация — выше приоритет).
// 3. Посты со статусом ближе к published (ready > draft > idea).
func (s *PostService) GetNextPost(ctx context.Context) (*Post, error) {
	posts, err := s.repo.List(ctx, PostFilter{})
	if err != nil {
		return nil, err
	}

	// Фильтруем только неопубликованные посты
	var candidates []*Post
	for _, post := range posts {
		if post.Status != StatusPublished && post.Status != StatusScheduled {
			candidates = append(candidates, post)
		}
	}

	if len(candidates) == 0 {
		return nil, nil //nolint:nilnil // Нет постов для рекомендации — это валидный результат.
	}

	// Сортируем по приоритету
	now := s.clock.Now()
	
	// Сортировка: сначала с дедлайном, потом со scheduled_at, потом по статусу
	sortPostsByPriority(candidates, now)

	return candidates[0], nil
}

// sortPostsByPriority сортирует посты по приоритету (на месте).
func sortPostsByPriority(posts []*Post, now time.Time) {
	// Простая пузырьковая сортировка для наглядности
	//nolint:intrange // Нужен классический цикл для пузырьковой сортировки
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
	// Приоритет 1: есть дедлайн
	if post.Deadline != nil {
		if post.Deadline.Before(now) {
			// Просроченный дедлайн — самый высокий приоритет
			return -1000 + int(now.Sub(*post.Deadline).Hours())
		}
		// Будущий дедлайн
		return int(post.Deadline.Sub(now).Hours())
	}

	// Приоритет 2: есть scheduled_at
	if post.ScheduledAt != nil {
		if post.ScheduledAt.Before(now) {
			// Уже должно было быть опубликовано
			return -500 + int(now.Sub(*post.ScheduledAt).Hours())
		}
		return 10000 + int(post.ScheduledAt.Sub(now).Hours())
	}

	// Приоритет 3: по статусу (ready=20000, draft=30000, idea=40000)
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
type PublishPostInput struct {
	Platforms []Platform
}

// PublishPost публикует пост на указанные платформы.
func (s *PostService) PublishPost(ctx context.Context, id PostID, input PublishPostInput, publishers map[Platform]Publisher) (*Post, error) {
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

	// Публикуем на каждую платформу
	for _, platform := range input.Platforms {
		publisher, ok := publishers[platform]
		if !ok {
			return nil, fmt.Errorf("%w: publisher для платформы %s не найден", ErrNotFound, platform)
		}

		updatedPost, err := publisher.Publish(ctx, post)
		if err != nil {
			return nil, fmt.Errorf("ошибка публикации в %s: %w", platform, err)
		}
		post = updatedPost
	}

	// Обновляем статус на published
	post.Status = StatusPublished
	now := s.clock.Now()
	post.PublishedAt = &now

	if err := s.repo.Update(ctx, post); err != nil {
		return nil, fmt.Errorf("ошибка обновления поста: %w", err)
	}

	return post, nil
}

// generateSlug генерирует slug из заголовка.
func (s *PostService) generateSlug(title string) string {
	return generateSlug(title)
}
