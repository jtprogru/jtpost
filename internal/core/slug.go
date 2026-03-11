package core

import (
	"strings"
	"unicode"
)

// transliterateMap карта транслитерации кириллицы в латиницу.
var transliterateMap = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d",
	'е': "e", 'ё': "yo", 'ж': "zh", 'з': "z", 'и': "i",
	'й': "y", 'к': "k", 'л': "l", 'м': "m", 'н': "n",
	'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t",
	'у': "u", 'ф': "f", 'х': "h", 'ц': "ts", 'ч': "ch",
	'ш': "sh", 'щ': "sch", 'ъ': "", 'ы': "y", 'ь': "",
	'э': "e", 'ю': "yu", 'я': "ya",
	'А': "A", 'Б': "B", 'В': "V", 'Г': "G", 'Д': "D",
	'Е': "E", 'Ё': "Yo", 'Ж': "Zh", 'З': "Z", 'И': "I",
	'Й': "Y", 'К': "K", 'Л': "L", 'М': "M", 'Н': "N",
	'О': "O", 'П': "P", 'Р': "R", 'С': "S", 'Т': "T",
	'У': "U", 'Ф': "F", 'Х': "H", 'Ц': "Ts", 'Ч': "Ch",
	'Ш': "Sh", 'Щ': "Sch", 'Ъ': "", 'Ы': "Y", 'Ь': "",
	'Э': "E", 'Ю': "Yu", 'Я': "Ya",
}

// Transliterate преобразует кириллический текст в латиницу.
// Использует стандартную систему транслитерации.
func Transliterate(text string) string {
	var result strings.Builder
	for _, r := range text {
		if trans, ok := transliterateMap[r]; ok {
			result.WriteString(trans)
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// generateSlug генерирует URL-безопасный slug из заголовка.
// Поддерживает кириллицу (транслитерирует) и латиницу.
func generateSlug(title string) string {
	// Транслитерируем кириллицу
	slug := Transliterate(title)

	// Приводим к нижнему регистру
	slug = strings.ToLower(slug)

	// Заменяем пробелы и подчёркивания на дефисы
	slug = strings.TrimSpace(slug)
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "_", "-")

	// Удаляем недопустимые символы (оставляем только a-z, 0-9, -)
	var result strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		} else if !unicode.IsSpace(r) {
			// Если символ не пробел и не допустимый — заменяем на дефис
			if result.Len() > 0 && result.String()[result.Len()-1] != '-' {
				result.WriteRune('-')
			}
		}
	}
	slug = result.String()

	// Удаляем повторяющиеся дефисы
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}

	// Обрезаем дефисы по краям
	slug = strings.Trim(slug, "-")

	return slug
}
