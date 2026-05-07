package fsrepo

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/core"
)

func samplePostForBench() *core.Post {
	return &core.Post{
		ID:        core.PostID(uuid.New()),
		TenantID:  uuid.New(),
		AuthorID:  uuid.New(),
		Title:     "Benchmark Post",
		Slug:      "benchmark-post",
		Status:    core.StatusDraft,
		Tags:      []string{"go", "bench", "fsrepo"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Revision:  1,
		Content:   strings.Repeat("Lorem ipsum dolor sit amet. ", 200),
	}
}

func BenchmarkSerializePost(b *testing.B) {
	p := samplePostForBench()
	b.ReportAllocs()
	for b.Loop() {
		_, _ = SerializePost(p)
	}
}

func BenchmarkParsePost(b *testing.B) {
	p := samplePostForBench()
	data, err := SerializePost(p)
	if err != nil {
		b.Fatalf("serialize: %v", err)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		_, _ = ParsePost(data)
	}
}
