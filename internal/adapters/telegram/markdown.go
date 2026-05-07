package telegram

import (
	"regexp"
	"strings"
)

// mdImageRegexp матчит markdown-image-syntax `![alt](url)`. URL — всё до закрывающей
// круглой скобки, alt — всё между `[` и `]`. Вложенные скобки в alt не поддерживаем
// (редкий кейс — для CMS-контента не критично).
var mdImageRegexp = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)

// MDImage — извлечённая картинка из markdown.
type MDImage struct {
	Alt string
	URL string
}

// ExtractImages находит все markdown-image references в тексте.
// Дубликаты (одинаковый URL) сохраняются в порядке первого вхождения.
func ExtractImages(md string) []MDImage {
	matches := mdImageRegexp.FindAllStringSubmatch(md, -1)
	out := make([]MDImage, 0, len(matches))
	seen := map[string]struct{}{}
	for _, m := range matches {
		url := strings.TrimSpace(m[2])
		if url == "" {
			continue
		}
		if _, ok := seen[url]; ok {
			continue
		}
		seen[url] = struct{}{}
		out = append(out, MDImage{Alt: m[1], URL: url})
	}
	return out
}

// StripImages удаляет все markdown-image-references из текста (для подготовки
// caption — Telegram не рендерит markdown-картинки внутри сообщения).
func StripImages(md string) string {
	return strings.TrimSpace(mdImageRegexp.ReplaceAllString(md, ""))
}
