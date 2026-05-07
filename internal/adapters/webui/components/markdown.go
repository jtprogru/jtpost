package components

import (
	"bytes"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// goldmark instance — без unsafe (raw HTML отбрасывается, autolink+gfm-tables).
//
//nolint:gochecknoglobals // singleton renderer; thread-safe.
var md = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
	),
	goldmark.WithParserOptions(
		parser.WithAutoHeadingID(),
	),
	goldmark.WithRendererOptions(
		html.WithHardWraps(),
		// Без html.WithUnsafe(): inline HTML в исходнике игнорируется → защита от XSS.
	),
)

// MarkdownToHTML рендерит Markdown в безопасный HTML. Inline HTML в источнике
// отбрасывается goldmark'ом по умолчанию; URL-схемы валидируются.
func MarkdownToHTML(src string) string {
	if src == "" {
		return ""
	}
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		return "<p><em>preview unavailable</em></p>"
	}
	return buf.String()
}
