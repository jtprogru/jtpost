package telegramconv

import (
	"strings"
	"testing"
)

const benchMarkdown = `# Заголовок

Текст с **жирным**, *курсивом* и ~~зачёркнутым~~. Также ` + "`inline`" + ` код.

## Подзаголовок

` + "```go\nfunc main() {\n    fmt.Println(\"hello\")\n}\n```" + `

[ссылка](https://example.com) и ещё немного текста.
`

func BenchmarkMarkdownToHTML_Small(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = MarkdownToHTML(benchMarkdown)
	}
}

func BenchmarkMarkdownToHTML_Large(b *testing.B) {
	large := strings.Repeat(benchMarkdown, 50)
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		_ = MarkdownToHTML(large)
	}
}

func BenchmarkFormatMessage(b *testing.B) {
	body := strings.Repeat("Текст с **жирным** и [ссылкой](https://example.com).\n", 30)
	b.ReportAllocs()
	for b.Loop() {
		_ = FormatMessage("Заголовок", body)
	}
}
