# Логирование в jtpost

## Обзор

Проект использует кастомный пакет логгера `internal/logger` для логирования событий в HTTP API и других компонентах.

## Уровни логирования

- **DEBUG** — детальная отладочная информация (только при включённом режиме `--verbose`)
- **INFO** — важная информация о выполнении операций
- **WARN** — предупреждения о некорректном поведении
- **ERROR** — ошибки, которые не позволяют выполнить операцию

## Конфигурация

### Режим отладки

Для включения DEBUG-логирования в HTTP API используйте флаг `--verbose`:

```bash
jtpost serve --verbose
```

В режиме отладки логгер выводит сообщения всех уровней, включая DEBUG.

## HTTP API логирование

### Middleware

HTTP сервер использует два middleware для логирования:

1. **LoggingMiddleware** — логирует каждый HTTP запрос:
   - Метод запроса (GET, POST, PUT, DELETE)
   - Путь запроса
   - HTTP статус ответа
   - Количество переданных байт
   - Длительность обработки

2. **RecoveryMiddleware** — восстанавливает сервер после паник и логирует стек трейс.

### Пример вывода

```
2026-03-12 12:00:00 INFO [HTTP] GET /api/posts 200 1234 5.234ms
2026-03-12 12:00:01 DEBUG [HTTP] POST /api/posts - создаём пост с заголовком "Test"
2026-03-12 12:00:01 INFO [HTTP] POST /api/posts 201 567 12.456ms
2026-03-12 12:00:02 WARN [HTTP] PUT /api/posts/123 - пост не найден
2026-03-12 12:00:02 ERROR [HTTP] DELETE /api/posts/456 - ошибка базы данных: connection refused
```

## Логирование в обработчиках

Каждый обработчик HTTP API логирует:

- **listPosts** — фильтры (статус, платформа, теги) и количество найденных постов
- **createPost** — заголовок и slug создаваемого поста
- **updatePost** — ID обновляемого поста и изменённые поля
- **deletePost** — ID удаляемого поста
- **getPost** — ID запрашиваемого поста
- **publishPost** — ID и платформу публикации
- **handleStats** — тип запрашиваемой статистики
- **handlePlan** — период планирования

## Использование логгера в коде

### Создание логгера

```go
import "github.com/jtprogru/jtpost/internal/logger"

log := logger.New(&logger.Config{
    Level:   logger.InfoLevel,
    Prefix:  "[HTTP]",
    Debug:   false,
})
```

### Логирование сообщений

```go
log.Info("запрос обработан успешно")
log.Debug("детали запроса: %v", requestData)
log.Warn("устаревший формат данных")
log.Error("ошибка подключения к БД: %v", err)
```

### Форматированные сообщения

```go
log.Infof("создан пост %s", post.Slug)
log.Debugf("параметры: %+v", params)
log.Warnf("повторная попытка %d из %d", attempt, maxAttempts)
log.Errorf("не удалось сохранить: %v", err)
```

## Расширение логирования

При добавлении новых компонентов:

1. Создайте экземпляр логгера с уникальным префиксом
2. Логируйте все важные операции (создание, обновление, удаление)
3. Логируйте ошибки с достаточным контекстом
4. Используйте DEBUG для детальной отладочной информации

## Примеры

### Логирование в обработчике

```go
func (s *Server) handleGetPost(w http.ResponseWriter, r *http.Request) {
    id := r.PathParam("id")
    
    s.log.Debugf("запрос поста %s", id)
    
    post, err := s.repo.GetByID(r.Context(), core.PostID(id))
    if err != nil {
        s.log.Warnf("пост %s не найден: %v", id, err)
        http.Error(w, "Post not found", http.StatusNotFound)
        return
    }
    
    s.log.Infof("пост %s получен успешно", id)
    json.NewEncoder(w).Encode(post)
}
```

### Логирование в middleware

```go
func LoggingMiddleware(next http.Handler, log *logger.Logger) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        
        rw := &responseWriter{ResponseWriter: w}
        next.ServeHTTP(rw, r)
        
        duration := time.Since(start)
        log.Infof("%s %s %d %d %v", 
            r.Method, 
            r.URL.Path, 
            rw.statusCode, 
            rw.bytesWritten,
            duration)
    })
}
```

## Отключение логирования

Для полного отключения логирования установите уровень `OffLevel`:

```go
log := logger.New(&logger.Config{
    Level: logger.OffLevel,
})
```

## Тестирование

Пакет `internal/logger` покрыт тестами. Для запуска:

```bash
go test -v ./internal/logger/...
```

## См. также

- [HTTP API Documentation](./api.md)
- [Architecture](../ROADMAP.md)
