package webui

import "strings"

// DiffOp — тип операции в построчном diff.
type DiffOp int

const (
	// DiffEqual — строка одинакова в обеих версиях.
	DiffEqual DiffOp = iota
	// DiffRemoved — строка есть в left (старой ревизии), удалена в right (текущей).
	DiffRemoved
	// DiffAdded — строка добавлена в right (текущей), не было в left.
	DiffAdded
)

// DiffRow — одна строка side-by-side таблицы.
type DiffRow struct {
	Op    DiffOp
	Left  string // содержимое слева (старая ревизия). Пусто если DiffAdded.
	Right string // содержимое справа (текущая). Пусто если DiffRemoved.
}

// DiffLines возвращает построчный side-by-side diff между left и right.
// Алгоритм — стандартный LCS на DP-таблице, O(n*m) по памяти и времени.
// Пригоден для постов до нескольких тысяч строк.
func DiffLines(left, right string) []DiffRow {
	a := splitLines(left)
	b := splitLines(right)
	n, m := len(a), len(b)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
				continue
			}
			dp[i][j] = max(dp[i-1][j], dp[i][j-1])
		}
	}
	rows := make([]DiffRow, 0, n+m)
	i, j := n, m
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && a[i-1] == b[j-1]:
			rows = append(rows, DiffRow{Op: DiffEqual, Left: a[i-1], Right: b[j-1]})
			i--
			j--
		case j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]):
			rows = append(rows, DiffRow{Op: DiffAdded, Right: b[j-1]})
			j--
		default:
			rows = append(rows, DiffRow{Op: DiffRemoved, Left: a[i-1]})
			i--
		}
	}
	// reverse
	for x, y := 0, len(rows)-1; x < y; x, y = x+1, y-1 {
		rows[x], rows[y] = rows[y], rows[x]
	}
	return rows
}

// splitLines разбивает строку на строки без trailing-empty (если оканчивается на \n).
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
