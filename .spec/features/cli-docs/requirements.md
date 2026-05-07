# cli-docs — `jtpost docs` команда + examples/

## Goal
Закрыть две позиции этапа 13 ROADMAP:
- ✅ Генерация документации CLI (`jtpost docs`)
- ✅ Примеры использования в `examples/`

## jtpost docs
- Команда `jtpost docs [--out=./docs/cli/]` генерирует Markdown-справку
  по всем подкомандам через `github.com/spf13/cobra/doc`.
- Дефолтный путь: `./docs/cli/`.
- Создаёт директорию если её нет; перезаписывает существующие файлы.
- Тестируется: запуск с `--out` в tmp-dir, проверка наличия `jtpost.md` и
  одного-двух subcmd-файлов.
- Сами сгенерированные docs коммитим в `docs/cli/` чтобы был текущий артефакт.

## examples/
- `examples/.jtpost.yaml` — минимальный production-like конфиг (sqlite + telegram).
- `examples/posts/sample-post.md` — пример frontmatter + body, демонстрирует
  все основные поля (title, status=draft, tags, deadline).
- `examples/README.md` — короткое объяснение что где.

## Out of scope
- Видео-туториалы.
- API documentation (OpenAPI уже есть в etap B.3).
- English README (отдельный cut если понадобится).
