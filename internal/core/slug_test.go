package core

import (
	"testing"
)

func TestTransliterate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Базовые тесты
		{"empty", "", ""},
		{"latin only", "Hello World", "Hello World"},
		{"numbers", "Test 123", "Test 123"},

		// Кириллица
		{"russian hello", "Привет", "Privet"},
		{"russian test", "Тест", "Test"},
		{"russian mixed", "Привет Мир", "Privet Mir"},

		// Полный алфавит
		{"lowercase a", "а", "a"},
		{"lowercase b", "б", "b"},
		{"lowercase v", "в", "v"},
		{"lowercase g", "г", "g"},
		{"lowercase d", "д", "d"},
		{"lowercase e", "е", "e"},
		{"lowercase yo", "ё", "yo"},
		{"lowercase zh", "ж", "zh"},
		{"lowercase z", "з", "z"},
		{"lowercase i", "и", "i"},
		{"lowercase y", "й", "y"},
		{"lowercase k", "к", "k"},
		{"lowercase l", "л", "l"},
		{"lowercase m", "м", "m"},
		{"lowercase n", "н", "n"},
		{"lowercase o", "о", "o"},
		{"lowercase p", "п", "p"},
		{"lowercase r", "р", "r"},
		{"lowercase s", "с", "s"},
		{"lowercase t", "т", "t"},
		{"lowercase u", "у", "u"},
		{"lowercase f", "ф", "f"},
		{"lowercase h", "х", "h"},
		{"lowercase ts", "ц", "ts"},
		{"lowercase ch", "ч", "ch"},
		{"lowercase sh", "ш", "sh"},
		{"lowercase sch", "щ", "sch"},
		{"lowercase hard sign", "ъ", ""},
		{"lowercase y letter", "ы", "y"},
		{"lowercase soft sign", "ь", ""},
		{"lowercase e reversed", "э", "e"},
		{"lowercase yu", "ю", "yu"},
		{"lowercase ya", "я", "ya"},

		// Uppercase
		{"uppercase A", "А", "A"},
		{"uppercase B", "Б", "B"},
		{"uppercase V", "В", "V"},
		{"uppercase G", "Г", "G"},
		{"uppercase D", "Д", "D"},
		{"uppercase E", "Е", "E"},
		{"uppercase YO", "Ё", "Yo"},
		{"uppercase ZH", "Ж", "Zh"},
		{"uppercase Z", "З", "Z"},
		{"uppercase I", "И", "I"},
		{"uppercase Y", "Й", "Y"},
		{"uppercase K", "К", "K"},
		{"uppercase L", "Л", "L"},
		{"uppercase M", "М", "M"},
		{"uppercase N", "Н", "N"},
		{"uppercase O", "О", "O"},
		{"uppercase P", "П", "P"},
		{"uppercase R", "Р", "R"},
		{"uppercase S", "С", "S"},
		{"uppercase T", "Т", "T"},
		{"uppercase U", "У", "U"},
		{"uppercase F", "Ф", "F"},
		{"uppercase H", "Х", "H"},
		{"uppercase TS", "Ц", "Ts"},
		{"uppercase CH", "Ч", "Ch"},
		{"uppercase SH", "Ш", "Sh"},
		{"uppercase SCH", "Щ", "Sch"},
		{"uppercase hard sign", "Ъ", ""},
		{"uppercase Y letter", "Ы", "Y"},
		{"uppercase soft sign", "Ь", ""},
		{"uppercase E reversed", "Э", "E"},
		{"uppercase YU", "Ю", "Yu"},
		{"uppercase YA", "Я", "Ya"},

		// Смешанные
		{"mixed Golang урок", "Golang урок", "Golang urok"},
		{"mixed Hello Привет", "Hello Привет", "Hello Privet"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Transliterate(tt.input)
			if result != tt.expected {
				t.Errorf("Transliterate(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGenerateSlug(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Базовые тесты
		{"empty", "", ""},
		{"single word", "Hello", "hello"},
		{"multiple words", "Hello World", "hello-world"},
		{"with numbers", "Test 123", "test-123"},

		// Пробелы и подчёркивания
		{"multiple spaces", "Multiple   Spaces", "multiple-spaces"},
		{"leading/trailing spaces", "  Trimmed  ", "trimmed"},
		{"underscores", "Hello_World", "hello-world"},
		{"mixed separators", "Hello _ World", "hello-world"},

		// Спецсимволы
		{"special chars", "Special!@#Chars", "special-chars"},
		{"with dots", "Hello.World", "hello-world"},
		{"with quotes", "Hello\"World", "hello-world"},

		// Кириллица
		{"cyrillic simple", "Тест", "test"},
		{"cyrillic hello", "Привет", "privet"},
		{"cyrillic multiple words", "Привет Мир", "privet-mir"},
		{"cyrillic with yo", "Ёжик", "yozhik"},
		{"cyrillic with zh", "Жизнь", "zhizn"},
		{"cyrillic with ts", "Отец", "otets"},
		{"cyrillic with ch", "Чай", "chay"},
		{"cyrillic with sh", "Шар", "shar"},
		{"cyrillic with sch", "Щука", "schuka"},
		{"cyrillic with yu", "Юг", "yug"},
		{"cyrillic with ya", "Яма", "yama"},
		{"cyrillic with hard sign", "Подъезд", "podezd"},
		{"cyrillic with soft sign", "Конь", "kon"},
		{"cyrillic with y", "Сыр", "syr"},

		// Смешанные
		{"mixed cyrillic latin", "Golang урок", "golang-urok"},
		{"mixed hello privet", "Hello Привет", "hello-privet"},
		{"mixed with numbers", "Тест 123", "test-123"},

		// Сложные случаи
		{"complex russian", "Как написать CLI на Go", "kak-napisat-cli-na-go"},
		{"complex with special", "Привет, Мир!", "privet-mir"},
		{"complex multiple dashes", "Тест---пост", "test-post"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateSlug(tt.input)
			if result != tt.expected {
				t.Errorf("generateSlug(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}
