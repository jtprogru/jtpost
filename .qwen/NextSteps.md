# Черновик структуры пакетов (`/cmd/jtpost`, `/internal/core`, `/internal/adapters/...`) и пример интерфейсов (`PostRepository`, `Publisher`) с сигнатурами

Ниже пример структуры в духе «cmd + internal + лёгкий хексагон», плюс наброски интерфейсов и доменных типов.[^1][^2][^3]

## Структура каталогов

```text
jtpost/
  cmd/
    jtpost/
      main.go

  internal/
    core/             # доменная модель и use-case'ы
      post.go
      status.go
      service.go
      publisher.go
      errors.go

    adapters/         # реализации портов (FS, Telegram, Hugo, HTTP)
      fsrepo/
        repository.go
      config/
        config.go
      telegram/
        publisher.go
      httpapi/
        server.go
        handlers.go

    cli/              # слой команд, "склейка" CLI <-> core
      root.go
      init.go
      new.go
      list.go
      status.go
      publish.go
      plan.go

  configs/
    jtpost.example.yaml

  # опционально:
  # /scripts, /docs, /testdata и т.п.
```

Такой layout опирается на стандартный подход `/cmd` + `/internal` для CLI‑приложений.[^4][^2][^1]

***

## Доменные типы (`internal/core`)

### Статусы и платформы

```go
package core

import "time"

type PostStatus string

const (
	StatusIdea      PostStatus = "idea"
	StatusDraft     PostStatus = "draft"
	StatusReady     PostStatus = "ready"
	StatusScheduled PostStatus = "scheduled"
	StatusPublished PostStatus = "published"
)

type Platform string

const (
	PlatformBlog     Platform = "blog"
	PlatformTelegram Platform = "telegram"
)
```


### Модель Post

```go
package core

type PostID string

type ExternalLinks struct {
	BlogURL     string
	TelegramURL string
}

type Post struct {
	ID          PostID
	Title       string
	Slug        string
	Status      PostStatus
	Platforms   []Platform
	Tags        []string

	Deadline    *time.Time
	ScheduledAt *time.Time
	PublishedAt *time.Time

	Content     string        // Markdown-тело
	Frontmatter map[string]any // сырые поля, чтобы не потерять ничего лишнего

	External ExternalLinks
}
```


***

## Интерфейсы портов (`PostRepository`, `Publisher`, `Clock`)

### PostRepository (файлы/БД)

```go
package core

import "context"

type PostFilter struct {
	Status    []PostStatus
	Platforms []Platform
	Tags      []string
	Before    *time.Time // для фильтрации по deadline / scheduledAt
	Search    string     // простенький full-text по title/slug
}

type PostRepository interface {
	GetByID(ctx context.Context, id PostID) (*Post, error)
	List(ctx context.Context, filter PostFilter) ([]*Post, error)

	// Создаёт новый пост (ID может быть сгенерирован тут или выше в сервисе)
	Create(ctx context.Context, p *Post) error
	Update(ctx context.Context, p *Post) error
}
```

Файловая реализация (`internal/adapters/fsrepo`) будет читать/писать Markdown + frontmatter в директорию.[^5][^6]

### Publisher (telegram и др.)

```go
package core

// Publisher публикует пост на конкретную платформу (telegram, ...)
// и возвращает обновлённый Post с заполненными ExternalLinks/Status.
type Publisher interface {
	Platform() Platform

	Publish(ctx context.Context, p *Post) (*Post, error)
}
```

Дальше будут конкретные адаптеры:

- `BlogPublisher` (Hugo/статик) в `internal/adapters/blog`.[^7][^8]
- `TelegramPublisher` в `internal/adapters/telegram`.[^9][^10]


### Clock (для тестируемого времени, планирования)

```go
package core

import "time"

type Clock interface {
	Now() time.Time
}
```

В проде — реализация через `time.Now()`, в тестах — фиксированное время.[^2][^3]

***

## Сервисный слой (`PostService`, `PlanningService`)

### PostService

```go
package core

import "context"

type PostService struct {
	repo  PostRepository
	clock Clock
}

func NewPostService(repo PostRepository, clock Clock) *PostService {
	return &PostService{repo: repo, clock: clock}
}

type NewPostInput struct {
	Title     string
	Platforms []Platform
	Tags      []string
	// возможно, шаблон/секция и т.п.
}

func (s *PostService) CreatePost(ctx context.Context, in NewPostInput) (*Post, error) {
	// генерируем slug, ID, проставляем StatusDraft или StatusIdea и т.д.
	// валидируем вход
	return nil, nil
}

func (s *PostService) UpdateStatus(ctx context.Context, id PostID, newStatus PostStatus) (*Post, error) {
	// правила перехода состояний (draft -> ready, ready -> scheduled/published и т.д.)
	return nil, nil
}

func (s *PostService) ListPosts(ctx context.Context, filter PostFilter) ([]*Post, error) {
	return s.repo.List(ctx, filter)
}
```


### PlanningService

```go
package core

type PlanningService struct {
	repo  PostRepository
	clock Clock
}

func NewPlanningService(repo PostRepository, clock Clock) *PlanningService {
	return &PlanningService{repo: repo, clock: clock}
}

type NextPostCriteria struct {
	Platforms []Platform
	Tags      []string
}

func (s *PlanningService) NextPostToWorkOn(ctx context.Context, c NextPostCriteria) (*Post, error) {
	// ищем по статусам idea/draft с ближайшим deadline и пр.
	return nil, nil
}
```


***

## Публикация (`PublishService`)

Чтобы CLI не знал о конкретных платформах, можно добавить слой, который управляет несколькими `Publisher`:

```go
package core

type PublishService struct {
	repo       PostRepository
	publishers map[Platform]Publisher
	clock      Clock
}

func NewPublishService(repo PostRepository, clock Clock, pubs ...Publisher) *PublishService {
	m := make(map[Platform]Publisher, len(pubs))
	for _, p := range pubs {
		m[p.Platform()] = p
	}
	return &PublishService{repo: repo, publishers: m, clock: clock}
}

func (s *PublishService) PublishToPlatform(ctx context.Context, id PostID, platform Platform) (*Post, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	pub, ok := s.publishers[platform]
	if !ok {
		return nil, ErrUnknownPlatform
	}
	updated, err := pub.Publish(ctx, p)
	if err != nil {
		return nil, err
	}
	// обновляем статус/время публикации
	updated.Status = StatusPublished
	now := s.clock.Now()
	updated.PublishedAt = &now
	return updated, s.repo.Update(ctx, updated)
}
```


***

## CLI слой (`internal/cli`)

В CLI слой импортирует только `core` и адаптеры, собирая всё в `*cobra.Command` (или другой фреймворк).[^3][^11]

Пример `cmd/jtpost/main.go`:

```go
package main

import "github.com/jtprogru/jtpost/internal/cli"

func main() {
	cli.Execute()
}
```

Пример корневой команды:

```go
package cli

import "github.com/spf13/cobra"

func Execute() {
	rootCmd := &cobra.Command{
		Use:   "jtpost",
		Short: "CLI-редактор постов для Telegram",
	}
	// сюда добавляются подкоманды new/list/publish/...
	rootCmd.Execute()
}
```

А в подкомандах ты создаёшь `PostService/PublishService`, конфиг, FS‑репозиторий и т.д.[^11][^1]

***

Если хочешь, следующим шагом можно конкретно расписать файловый репозиторий (`fsrepo`): как парсить/писать frontmatter + Markdown, и какой формат ID/пути к файлам выбрать, чтобы не пожалеть через полгода.

## Список источников

[^1]: <https://github.com/golang-standards/project-layout>
[^2]: <https://www.alexedwards.net/blog/11-tips-for-structuring-your-go-projects>
[^3]: <https://www.bytesizego.com/blog/structure-go-cli-app>
[^4]: <https://go.dev/doc/modules/layout>
[^5]: <https://mortenvistisen.com/posts/how-to-create-a-blog-using-golang>
[^6]: <https://dev.to/envitab/how-to-create-a-static-site-generator-with-go-4jgm>
[^7]: <https://www.geeksforgeeks.org/go-language/static-site-generation-with-hugo/>
[^8]: <https://sambloomquist.com/posts/publishing-hugo-static-site-github-pages/>
[^9]: <https://postoplan.contenive.com/scheduled-posting-on-telegram>
[^10]: <https://amarketsaffiliates.com/setting-up-automated-posting-on-telegram/>
[^11]: <https://dev.to/rinkiyakedad/creating-a-cli-in-golang-5abl>
