# benchmarks — горячие участки

## Goal
Закрыть "Benchmark тесты для критичных участков" из ROADMAP §380.

## Scope
Бенчмарки для функций, которые:
1. Работают с потенциально большими входами (diff, frontmatter parse).
2. Вызываются на каждом запросе/публикации (markdown → telegram-convert).
3. Имеют известный сложностный класс, который полезно подтвердить.

### Цели
- `webui.DiffLines` — O(n*m) LCS, проверить деградацию на 100/500/1000 строк.
- `fsrepo.ParsePost` / `fsrepo.SerializePost` — frontmatter round-trip.
- `telegramconv.MarkdownToHTML` — вызывается перед каждым sendMessage.

## Out of scope
- CI integration (запускать бенчи в CI).
- Performance regressions tracking (benchstat baseline).
- Профилирование/оптимизация — отдельный cut если найдём узкое место.
