package webui

import (
	"strings"
	"testing"
)

func makeLines(n int, prefix string) string {
	var b strings.Builder
	for i := range n {
		b.WriteString(prefix)
		b.WriteString(" line ")
		writeInt(&b, i)
		b.WriteByte('\n')
	}
	return b.String()
}

func writeInt(b *strings.Builder, n int) {
	if n == 0 {
		b.WriteByte('0')
		return
	}
	var digits [20]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	b.Write(digits[i:])
}

func BenchmarkDiffLines_Identical_100(b *testing.B) {
	left := makeLines(100, "x")
	for b.Loop() {
		_ = DiffLines(left, left)
	}
}

func BenchmarkDiffLines_Identical_500(b *testing.B) {
	left := makeLines(500, "x")
	for b.Loop() {
		_ = DiffLines(left, left)
	}
}

func BenchmarkDiffLines_Identical_1000(b *testing.B) {
	left := makeLines(1000, "x")
	for b.Loop() {
		_ = DiffLines(left, left)
	}
}

func BenchmarkDiffLines_FullyDifferent_500(b *testing.B) {
	left := makeLines(500, "old")
	right := makeLines(500, "new")
	for b.Loop() {
		_ = DiffLines(left, right)
	}
}

func BenchmarkDiffLines_HalfDifferent_500(b *testing.B) {
	leftLines := strings.Split(makeLines(500, "x"), "\n")
	rightLines := append([]string(nil), leftLines...)
	for i := 0; i < len(rightLines); i += 2 {
		rightLines[i] = "changed " + rightLines[i]
	}
	left := strings.Join(leftLines, "\n")
	right := strings.Join(rightLines, "\n")
	b.ResetTimer()
	for b.Loop() {
		_ = DiffLines(left, right)
	}
}
