// Package telegramconv предоставляет конвертеры Markdown в Telegram HTML.
package telegramconv

import (
	"fmt"
	"regexp"
	"strings"
)

// MarkdownToHTML конвертирует Markdown разметку в Telegram HTML формат.
// Telegram поддерживает ограниченный набор HTML тегов:
// <b>, <i>, <u>, <s>, <spoiler>, <a>, <code>, <pre>.
func MarkdownToHTML(md string) string {
	// Блок кода ```code``` → <pre>code</pre> (должен быть первым)
	md = convertCodeBlocks(md)

	// Жирный текст **text** → <b>text</b>
	md = convertBold(md)

	// Курсив *text* или _text_ → <i>text</i>
	md = convertItalic(md)

	// Зачёркивание ~~text~~ → <s>text</s>
	md = convertStrikethrough(md)

	// Инлайн код `code` → <code>code</code>
	md = convertInlineCode(md)

	// Ссылки [text](url) → <a href="url">text</a>
	md = convertLinks(md)

	return md
}

// convertBold заменяет **text** на <b>text</b>.
func convertBold(md string) string {
	re := regexp.MustCompile(`\*\*(.+?)\*\*`)
	return re.ReplaceAllString(md, "<b>$1</b>")
}

// convertItalic заменяет *text* и _text_ на <i>text</i>.
func convertItalic(md string) string {
	// Обрабатываем *text* → <i>text</i>
	// Жирный текст (**text**) уже заменён, поэтому в строке нет двойных звёздочек.
	// Паттерн \*([^*]+?)\* берёт самое короткое содержимое между одиночными звёздочками.
	re := regexp.MustCompile(`\*([^*]+?)\*`)
	md = re.ReplaceAllString(md, "<i>$1</i>")

	// Затем обрабатываем _text_ → <i>text</i>.
	// Используем границы слова \b, чтобы не захватывать подчёркивания внутри слов.
	re2 := regexp.MustCompile(`\b_([^_]+?)_\b`)
	return re2.ReplaceAllString(md, "<i>$1</i>")
}

// convertStrikethrough заменяет ~~text~~ на <s>text</s>.
func convertStrikethrough(md string) string {
	re := regexp.MustCompile(`~~(.+?)~~`)
	return re.ReplaceAllString(md, "<s>$1</s>")
}

// convertInlineCode заменяет `code` на <code>code</code>.
// HTML-символы внутри кода экранируются для безопасного отображения.
func convertInlineCode(md string) string {
	re := regexp.MustCompile("`([^`]+)`")
	return re.ReplaceAllStringFunc(md, func(match string) string {
		// Извлекаем содержимое между обратными кавычками
		content := match[1 : len(match)-1]
		// Экранируем HTML-символы
		escaped := EscapeHTML(content)
		return "<code>" + escaped + "</code>"
	})
}

// convertCodeBlocks заменяет ```code``` на <pre>code</pre>.
// HTML-символы внутри кода экранируются для безопасного отображения.
func convertCodeBlocks(md string) string {
	re := regexp.MustCompile("```([\\s\\S]+?)```")
	return re.ReplaceAllStringFunc(md, func(match string) string {
		// Извлекаем содержимое между тройными кавычками
		content := match[3 : len(match)-3]
		// Экранируем HTML-символы
		escaped := EscapeHTML(content)
		return "<pre>" + escaped + "</pre>"
	})
}

// convertLinks заменяет [text](url) на <a href="url">text</a>.
func convertLinks(md string) string {
	re := regexp.MustCompile(`\[(.+?)\]\((.+?)\)`)
	return re.ReplaceAllString(md, `<a href="$2">$1</a>`)
}

// EscapeHTML экранирует HTML спецсимволы для безопасного отображения.
func EscapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// FormatMessage форматирует полное сообщение с заголовком.
func FormatMessage(title, content string) string {
	htmlContent := MarkdownToHTML(content)
	return fmt.Sprintf("<b>%s</b>\n\n%s", EscapeHTML(title), htmlContent)
}
