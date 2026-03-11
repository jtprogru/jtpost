package telegramconv

import (
	"testing"
)

func TestMarkdownToHTML(t *testing.T) {
	tests := []struct {
		name     string
		markdown string
		expected string
	}{
		{
			name:     "bold text",
			markdown: "**bold text**",
			expected: "<b>bold text</b>",
		},
		{
			name:     "italic text with asterisks",
			markdown: "*italic text*",
			expected: "<i>italic text</i>",
		},
		{
			name:     "italic text with underscores",
			markdown: "_italic text_",
			expected: "<i>italic text</i>",
		},
		{
			name:     "strikethrough text",
			markdown: "~~strikethrough~~",
			expected: "<s>strikethrough</s>",
		},
		{
			name:     "inline code",
			markdown: "`code here`",
			expected: "<code>code here</code>",
		},
		{
			name: "code block",
			markdown: "```go\nfmt.Println(\"hello\")\n```",
			expected: "<pre>go\nfmt.Println(\"hello\")\n</pre>",
		},
		{
			name:     "link",
			markdown: "[Google](https://google.com)",
			expected: `<a href="https://google.com">Google</a>`,
		},
		{
			name: "mixed formatting",
			markdown: "**bold** and *italic* and `code`",
			expected: "<b>bold</b> and <i>italic</i> and <code>code</code>",
		},
		{
			name: "complex post",
			markdown: `# Title

**Important** text with *italics*.

Check this:
` + "```python\ndef hello():\n    print(\"world\")\n```" + `

Visit [our site](https://example.com) for more.`,
			expected: `# Title

<b>Important</b> text with <i>italics</i>.

Check this:
<pre>python
def hello():
    print("world")
</pre>

Visit <a href="https://example.com">our site</a> for more.`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MarkdownToHTML(tt.markdown)
			if result != tt.expected {
				t.Errorf("MarkdownToHTML() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestEscapeHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "ampersand",
			input:    "A & B",
			expected: "A &amp; B",
		},
		{
			name:     "less than",
			input:    "1 < 2",
			expected: "1 &lt; 2",
		},
		{
			name:     "greater than",
			input:    "2 > 1",
			expected: "2 &gt; 1",
		},
		{
			name:     "all special chars",
			input:    "A & B < C > D",
			expected: "A &amp; B &lt; C &gt; D",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EscapeHTML(tt.input)
			if result != tt.expected {
				t.Errorf("EscapeHTML() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFormatMessage(t *testing.T) {
	title := "Test Title"
	content := "**bold** and *italic*"
	expected := "<b>Test Title</b>\n\n<b>bold</b> and <i>italic</i>"

	result := FormatMessage(title, content)
	if result != expected {
		t.Errorf("FormatMessage() = %q, want %q", result, expected)
	}
}

func TestConvertBold(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"single bold", "**hello**", "<b>hello</b>"},
		{"multiple bold", "**one** and **two**", "<b>one</b> and <b>two</b>"},
		{"bold with spaces", "**hello world**", "<b>hello world</b>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertBold(tt.input)
			if result != tt.expected {
				t.Errorf("convertBold() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestConvertItalic(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"single italic asterisk", "*hello*", "<i>hello</i>"},
		{"single italic underscore", "_hello_", "<i>hello</i>"},
		{"multiple italic", "*one* and _two_", "<i>one</i> and <i>two</i>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertItalic(tt.input)
			if result != tt.expected {
				t.Errorf("convertItalic() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestConvertStrikethrough(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"single strikethrough", "~~hello~~", "<s>hello</s>"},
		{"multiple strikethrough", "~~one~~ and ~~two~~", "<s>one</s> and <s>two</s>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertStrikethrough(tt.input)
			if result != tt.expected {
				t.Errorf("convertStrikethrough() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestConvertInlineCode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"single inline code", "`hello`", "<code>hello</code>"},
		{"multiple inline code", "`one` and `two`", "<code>one</code> and <code>two</code>"},
		{"code with special chars", "`a < b`", "<code>a &lt; b</code>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertInlineCode(tt.input)
			if result != tt.expected {
				t.Errorf("convertInlineCode() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestConvertCodeBlocks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "single code block",
			input: "```go\nfmt.Println(\"hi\")\n```",
			expected: "<pre>go\nfmt.Println(\"hi\")\n</pre>",
		},
		{
			name: "code block without language",
			input: "```\nsome code\n```",
			expected: "<pre>\nsome code\n</pre>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertCodeBlocks(tt.input)
			if result != tt.expected {
				t.Errorf("convertCodeBlocks() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestConvertLinks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"single link", "[Google](https://google.com)", `<a href="https://google.com">Google</a>`},
		{"multiple links", "[One](https://one.com) and [Two](https://two.com)", `<a href="https://one.com">One</a> and <a href="https://two.com">Two</a>`},
		{"link with special chars", "[A & B](https://example.com)", `<a href="https://example.com">A & B</a>`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertLinks(tt.input)
			if result != tt.expected {
				t.Errorf("convertLinks() = %q, want %q", result, tt.expected)
			}
		})
	}
}
