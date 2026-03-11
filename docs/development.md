# Руководство для разработчиков jtpost

## Обзор

Это руководство описывает процесс разработки, тестирования и внесения изменений в проект **jtpost**.

## Требования

### Обязательные

- **Go** 1.25.5 или выше
- **Git** для контроля версий
- **Task** (опционально, для удобства)

### Опциональные

- **golangci-lint** — для линтинга
- **goreleaser** — для сборки релизов

### Установка зависимостей

```bash
# Установка Task (macOS)
brew install go-task

# Установка golangci-lint
brew install golangci-lint

# Установка goreleaser
brew install goreleaser
```

## Структура проекта

```
jtpost/
├── cmd/jtpost/           # Точка входа CLI
├── internal/
│   ├── core/             # Доменная логика и интерфейсы
│   ├── adapters/         # Реализации (FS, Telegram, HTTP)
│   └── cli/              # Cobra команды
├── templates/            # Шаблоны постов
├── testdata/             # Тестовые данные
├── docs/                 # Документация
└── Taskfile.yml          # Задачи разработки
```

## Сборка и запуск

### Через Task

```bash
# Запуск через go run
task run:cmd

# Сборка бинарника
task build:bin

# Сборка с указанием версии
task build:bin version=0.2.0 commit=abc123
```

### Вручную

```bash
# Запуск
go run cmd/jtpost/main.go

# Сборка
CGO_ENABLED=0 go build -o ./dist/jtpost cmd/jtpost/main.go

# Сборка с версией
CGO_ENABLED=0 go build -ldflags="-X main.version=0.2.0" -o ./dist/jtpost cmd/jtpost/main.go
```

## Тестирование

### Запуск тестов

```bash
# Все тесты
task test

# Тесты с race detector
task test:race

# Тесты с покрытием
task test:coverage

# Открыть отчёт о покрытии
go tool cover -html=cover.out
```

### Вручную

```bash
# Запуск тестов
go test -v ./...

# С покрытием
go test -coverprofile=cover.out -v ./...

# Race detector
go test -race -v ./...

# Конкретный пакет
go test -v ./internal/core/
```

### Написание тестов

**Юнит-тест на сервис:**

```go
// internal/core/service_test.go
func TestPostService_CreatePost(t *testing.T) {
    repo := NewMockRepository()
    clock := NewMockClock()
    svc := NewPostService(repo, clock)

    post, err := svc.CreatePost(context.Background(), CreatePostInput{
        Title: "Test Post",
    })

    require.NoError(t, err)
    assert.Equal(t, "Test Post", post.Title)
    assert.Equal(t, StatusIdea, post.Status)
}
```

**Тест репозитория с временной директорией:**

```go
// internal/adapters/fsrepo/repository_test.go
func TestFileSystemRepository_Create(t *testing.T) {
    dir := t.TempDir()
    repo, err := NewFileSystemRepository(dir)
    require.NoError(t, err)

    post := &core.Post{
        ID:    "test-id",
        Title: "Test",
    }

    err = repo.Create(context.Background(), post)
    require.NoError(t, err)

    // Проверка что файл создан
    _, err = os.Stat(filepath.Join(dir, "test-id.md"))
    require.NoError(t, err)
}
```

## Линтинг

### Запуск линтера

```bash
# Через Task
task lint

# Вручную
golangci-lint run -v
```

### Конфигурация

Проект использует `.golangci.yaml` с набором линтеров:

- **Основные:** `staticcheck`, `errcheck`, `gosec`, `ineffassign`, `unused`
- **Стиль:** `gochecknoglobals`, `gochecknoinits`, `godot`, `misspell`
- **Тесты:** `testifylint`, `thelper`, `tparallel`

### Исправление проблем

```bash
# Автоматическое исправление
golangci-lint run --fix

# Форматирование
gofmt -s -w .
```

## Форматирование кода

```bash
# Через Task
task fmt

# Вручную
gofmt -s -w .
goimports -w .
```

## Внесение изменений

### Ветка для разработки

```bash
# Создать ветку от main
git checkout main
git pull
git checkout -b feature/new-feature

# Или ветка для исправления
git checkout -b fix/bug-fix
```

### Коммиты

```bash
# Проверка статуса
git status

# Просмотр изменений
git diff

# Добавление файлов
git add <files>

# Коммит
git commit -m "feat: описание изменений"
```

### Соглашения по коммитам

**Формат:**
```
<type>: <description>

[optional body]

[optional footer]
```

**Типы:**
- `feat` — новая функциональность
- `fix` — исправление бага
- `docs` — изменение документации
- `style` — форматирование, отступы
- `refactor` — рефакторинг без изменений функциональности
- `test` — добавление/изменение тестов
- `chore` — изменение конфигурации сборки, зависимости

**Примеры:**
```
feat: добавить команду stats для статистики постов

fix: исправить ошибку парсинга frontmatter с пустыми тегами

docs: обновить документацию CLI

refactor: вынести конвертацию Markdown в отдельный пакет
```

## Создание новой команды CLI

### 1. Создать файл команды

```go
// internal/cli/stats.go
package cli

import (
    "fmt"
    "github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
    Use:   "stats",
    Short: "Статистика по постам",
    Long:  `Выводит статистику по постам: количество по статусам, платформам и тегам.`,
    RunE: func(cmd *cobra.Command, args []string) error {
        // Логика команды
        return nil
    },
}

func init() {
    rootCmd.AddCommand(statsCmd)
}
```

### 2. Добавить в root.go

Убедитесь что команда добавлена в `rootCmd.AddCommand()` в `internal/cli/root.go`.

### 3. Написать тесты

```go
// internal/cli/stats_test.go
package cli

import (
    "testing"
    "github.com/stretchr/testify/require"
)

func TestStatsCmd(t *testing.T) {
    // Тест логики команды
}
```

### 4. Обновить документацию

Добавьте описание команды в `docs/cli.md`.

## Создание нового адаптера

### 1. Определить интерфейс в core

```go
// internal/core/publisher.go
type Publisher interface {
    Platform() Platform
    Publish(ctx context.Context, post *Post) (*Post, error)
}
```

### 2. Создать адаптер

```go
// internal/adapters/telegram/publisher.go
package telegram

import (
    "context"
    "github.com/jtprogru/jtpost/internal/core"
)

type Publisher struct {
    // поля
}

func NewPublisher(cfg Config) *Publisher {
    return &Publisher{}
}

func (p *Publisher) Platform() core.Platform {
    return core.PlatformTelegram
}

func (p *Publisher) Publish(ctx context.Context, post *core.Post) (*core.Post, error) {
    // Логика публикации
    return post, nil
}
```

### 3. Написать тесты

```go
// internal/adapters/telegram/publisher_test.go
package telegram

import (
    "testing"
    "github.com/stretchr/testify/require"
)

func TestPublisher_Publish(t *testing.T) {
    // Тест публикации
}
```

## Работа с доменной моделью

### Добавление нового поля Post

**1. Изменить структуру:**

```go
// internal/core/post.go
type Post struct {
    // ...
    Author string `yaml:"author,omitempty" json:"author,omitempty"`
}
```

**2. Обновить парсинг frontmatter:**

```go
// internal/adapters/fsrepo/repository.go
// Парсинг автоматически работает через yaml.Unmarshal
```

**3. Обновить CLI команды:**

```go
// internal/cli/show.go
fmt.Printf("  Автор:    %s\n", post.Author)
```

**4. Обновить API:**

```go
// internal/adapters/httpapi/server.go
type jsonPost struct {
    // ...
    Author string `json:"author,omitempty"`
}
```

**5. Обновить Web UI:**

```html
<!-- internal/adapters/httpapi/templates/index.html -->
<input type="text" id="post-author"/>
```

## Отладка

### Логирование

Используйте `fmt.Println` для отладки или добавьте флаг `--verbose`:

```go
// internal/cli/root.go
var verbose bool

func init() {
    rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "подробный вывод")
}

// В коде
if verbose {
    fmt.Printf("DEBUG: загружено %d постов\n", len(posts))
}
```

### Delve отладчик

```bash
# Установка
go install github.com/go-delve/delve/cmd/dlv@latest

# Запуск отладчика
dlv debug ./cmd/jtpost

# Breakpoint
(dlv) break main.main
(dlv) continue
```

## Сборка релиза

### Через goreleaser

```bash
# Snapshot сборка
goreleaser release --snapshot --clean

# Продакшен релиз
goreleaser release --clean
```

### Вручную

```bash
# Кросс-компиляция
GOOS=linux GOARCH=amd64 go build -o dist/jtpost-linux-amd64 ./cmd/jtpost
GOOS=darwin GOARCH=amd64 go build -o dist/jtpost-darwin-amd64 ./cmd/jtpost
GOOS=windows GOARCH=amd64 go build -o dist/jtpost-windows-amd64.exe ./cmd/jtpost
```

## Зависимости

### Добавление зависимости

```bash
go get github.com/pkg/errors@v0.9.1
go mod tidy
```

### Обновление зависимостей

```bash
go get -u ./...
go mod tidy
```

### Проверка зависимостей

```bash
go mod verify
go mod why github.com/pkg/errors
```

## CI/CD

### GitHub Actions

Проект использует GitHub Actions для:
- Линтинга кода
- Запуска тестов
- Сборки релизов

**Workflow файлы:**
- `.github/workflows/ci.yml` — CI пайплайн
- `.github/workflows/release.yml` — релизы

### Локальная проверка

```bash
# Запустить те же команды что и в CI
task lint
task test
task build:bin
```

## Документирование

### Обновление документации

1. **CLI команды** → `docs/cli.md`
2. **API endpoints** → `docs/api.md`
3. **Архитектура** → `docs/architecture.md`
4. **Конфигурация** → `docs/configuration.md`
5. **README** → `README.md`

### Godoc комментарии

```go
// PostService управляет жизненным циклом постов.
type PostService struct {
    repo  PostRepository
    clock Clock
}

// NewPostService создаёт новый PostService.
func NewPostService(repo PostRepository, clock Clock) *PostService {
    return &PostService{repo: repo, clock: clock}
}
```

### Проверка документации

```bash
godoc -http=:6060
# Открыть http://localhost:6060/pkg/github.com/jtprogru/jtpost
```

## Производительность

### Бенчмарки

```go
// internal/core/service_test.go
func BenchmarkCreatePost(b *testing.B) {
    repo := NewMockRepository()
    svc := NewPostService(repo, &MockClock{})

    for i := 0; i < b.N; i++ {
        svc.CreatePost(context.Background(), CreatePostInput{
            Title: fmt.Sprintf("Post %d", i),
        })
    }
}
```

```bash
# Запуск бенчмарков
go test -bench=. -benchmem ./...

# Профилирование
go test -cpuprofile=cpu.prof -memprofile=mem.prof ./...
```

### Анализ профиля

```bash
go tool pprof cpu.prof
go tool pprof mem.prof
```

## Безопасность

### Чувствительные данные

- ❌ Не коммитьте `.jtpost.yaml` с токенами
- ✅ Используйте переменные окружения
- ✅ Добавьте `.jtpost.yaml` в `.gitignore`

### Проверка уязвимостей

```bash
# Установка govulncheck
go install golang.org/x/vuln/cmd/govulncheck@latest

# Проверка проекта
govulncheck ./...
```

## Чек-лист перед коммитом

- [ ] Код отформатирован (`gofmt -s -w .`)
- [ ] Линтер проходит (`golangci-lint run`)
- [ ] Тесты проходят (`go test ./...`)
- [ ] Покрытие не уменьшилось
- [ ] Документация обновлена
- [ ] Коммит следует соглашениям

## Чек-лист перед релизом

- [ ] Все тесты проходят
- [ ] Линтер без ошибок
- [ ] Документация актуальна
- [ ] CHANGELOG обновлён
- [ ] Версия обновлена (semver)
- [ ] Релизный тег создан

## Troubleshooting

### Ошибка: "module declares its path as ..."

```bash
go clean -modcache
go mod tidy
```

### Ошибка: "imported and not used"

```bash
goimports -w .
```

### Тесты падают с race condition

```bash
# Найти гонку
go test -race ./...

# Исправить использование общих переменных в тестах
```

### golangci-lint выдаёт ошибки

```bash
# Посмотреть детали
golangci-lint run -v

# Автоматическое исправление
golangci-lint run --fix
```

## Полезные ссылки

- [Effective Go](https://go.dev/doc/effective_go)
- [Go Blog](https://go.dev/blog/)
- [Cobra Documentation](https://github.com/spf13/cobra)
- [golangci-lint](https://golangci-lint.run/)
- [Go Test Tutorial](https://go.dev/doc/tutorial/add-a-test)
