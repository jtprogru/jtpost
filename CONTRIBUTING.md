# Вклад в проект jtpost

Спасибо за интерес к проекту **jtpost**! Этот документ описывает процесс внесения изменений в проект.

## 📋 Содержание

- [Начало работы](#начало-работы)
- [Правила разработки](#правила-разработки)
- [Стиль кода](#стиль-кода)
- [Тестирование](#тестирование)
- [Создание Pull Request](#создание-pull-request)
- [Code Review](#code-review)

---

## Начало работы

### 1. Форк репозитория

Нажмите кнопку **Fork** на GitHub для создания копии проекта в вашем аккаунте.

### 2. Клонирование

```bash
git clone https://github.com/YOUR_USERNAME/jtpost.git
cd jtpost
```

### 3. Настройка Go

```bash
go version  # Должна быть 1.25.5+
go mod download
```

### 4. Установка инструментов

```bash
# golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Task (опционально)
go install github.com/go-task/task/v3/cmd/task@latest
```

---

## Правила разработки

### Ветвление

- **main** — основная ветка, стабильная версия
- **feature/<name>** — новые функции
- **bugfix/<name>** — исправление багов
- **docs/<name>** — документация
- **refactor/<name>** — рефакторинг

### Названия коммитов

Используйте [Conventional Commits](https://www.conventionalcommits.org/):

- `feat: добавить новую функцию`
- `fix: исправить баг с публикацией`
- `docs: обновить README`
- `refactor: упростить логику сервиса`
- `test: добавить тесты для CLI`
- `chore: обновить зависимости`

Примеры:

```bash
feat: добавить поддержку мультимедиа в Telegram
fix: исправить ошибку парсинга frontmatter
docs: добавить примеры использования CLI
refactor: вынести конвертеры в отдельный пакет
test: покрыть тестами PostService
chore: обновить Go до 1.26
```

---

## Стиль кода

### Go

- Следовать [Effective Go](https://go.dev/doc/effective_go)
- Использовать `gofmt -s -w .` перед коммитом
- Избегать `interface{}`, использовать `any`
- Именовать переменные в `camelCase`, константы в `PascalCase`

### Линтинг

Перед отправкой кода убедитесь, что линтер проходит:

```bash
golangci-lint run
```

Или через Task:

```bash
task lint
```

### Структура кода

- **internal/core/** — доменная модель, интерфейсы
- **internal/adapters/** — реализации (FS, SQLite, HTTP, Telegram)
- **internal/cli/** — Cobra команды
- **cmd/jtpost/** — точка входа CLI

---

## Тестирование

### Запуск тестов

```bash
# Все тесты
go test ./...

# С race detector
go test -race ./...

# С покрытием
go test -coverprofile=cover.out ./...

# Открыть отчёт
go tool cover -html=cover.out
```

### Написание тестов

- Использовать `t.TempDir()` для временных файлов
- Mock интерфейсов через `internal/core/mocks`
- Именовать тесты: `Test<Функция>_<Сценарий>`

Пример:

```go
func TestPostService_CreatePost_ValidData(t *testing.T) {
    t.Parallel()
    
    // Arrange
    repo := mocks.NewMockPostRepository()
    service := core.NewPostService(repo, clock)
    
    // Act
    post, err := service.CreatePost(context.Background(), "Title")
    
    // Assert
    assert.NoError(t, err)
    assert.NotNil(t, post)
}
```

---

## Создание Pull Request

### 1. Создать ветку

```bash
git checkout -b feature/my-feature
```

### 2. Внести изменения

Сделать коммиты с понятными сообщениями.

### 3. Обновить документацию

Если изменилась функциональность, обновите:

- `README.md`
- `docs/cli.md`
- `ROADMAP.md`

### 4. Проверить CI

Убедитесь, что все проверки проходят:

```bash
# Локальная проверка
task lint
task test
task build:bin
```

### 5. Отправить изменения

```bash
git push origin feature/my-feature
```

### 6. Создать PR

На GitHub создайте Pull Request с описанием изменений.

---

## Code Review

### Требования к PR

- [ ] Код отформатирован
- [ ] Линтер проходит без ошибок
- [ ] Все тесты проходят
- [ ] Добавлены тесты для новых функций
- [ ] Обновлена документация

### Процесс ревью

1. **Автоматические проверки** — CI запускает тесты и линтер
2. **Ревью от мейнтейнера** — проверка кода и архитектуры
3. **Исправление замечаний** — внесение правок
4. **Мерж** — слияние в основную ветку

### Критерии приёмки

- ✅ Код соответствует стандартам Go
- ✅ Нет дублирования логики
- ✅ Обработаны ошибки
- ✅ Покрытие тестами достаточное
- ✅ Документация актуальна

---

## Вопросы

### Где задать вопрос?

- **GitHub Issues** — для вопросов по функциональности
- **GitHub Discussions** — для общих обсуждений

### Как сообщить о баге?

Создайте issue с шаблоном **Bug Report**.

### Как предложить функцию?

Создайте issue с шаблоном **Feature Request**.

---

## Лицензия

MIT — см. файл [LICENSE](LICENSE).
