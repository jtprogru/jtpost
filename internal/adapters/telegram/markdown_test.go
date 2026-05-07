package telegram

import (
	"strings"
	"testing"
)

func TestExtractImages_Basic(t *testing.T) {
	t.Parallel()
	md := "Hello ![cat](https://example.com/cat.jpg) world ![dog](https://example.com/dog.png)"
	imgs := ExtractImages(md)
	if len(imgs) != 2 {
		t.Fatalf("got %d images, want 2", len(imgs))
	}
	if imgs[0].URL != "https://example.com/cat.jpg" || imgs[0].Alt != "cat" {
		t.Errorf("first: %+v", imgs[0])
	}
	if imgs[1].URL != "https://example.com/dog.png" || imgs[1].Alt != "dog" {
		t.Errorf("second: %+v", imgs[1])
	}
}

func TestExtractImages_Deduplicates(t *testing.T) {
	t.Parallel()
	md := "![a](https://x/y.png) and again ![b](https://x/y.png)"
	imgs := ExtractImages(md)
	if len(imgs) != 1 {
		t.Errorf("expected 1 (dedup), got %d", len(imgs))
	}
}

func TestExtractImages_LocalPaths(t *testing.T) {
	t.Parallel()
	md := "![](/ui/uploads/2026/05/abc.png)"
	imgs := ExtractImages(md)
	if len(imgs) != 1 || imgs[0].URL != "/ui/uploads/2026/05/abc.png" {
		t.Errorf("got %+v", imgs)
	}
}

func TestExtractImages_WithTitle(t *testing.T) {
	t.Parallel()
	md := `![alt](https://x/y.png "Title text")`
	imgs := ExtractImages(md)
	if len(imgs) != 1 || imgs[0].URL != "https://x/y.png" {
		t.Errorf("title-syntax not handled: %+v", imgs)
	}
}

func TestExtractImages_Empty(t *testing.T) {
	t.Parallel()
	if got := ExtractImages(""); len(got) != 0 {
		t.Errorf("empty md → got %d", len(got))
	}
	if got := ExtractImages("just text, no images"); len(got) != 0 {
		t.Errorf("plain text → got %d", len(got))
	}
}

func TestExtractImages_IgnoresLinks(t *testing.T) {
	t.Parallel()
	md := "[link text](https://x/y) is not an image"
	if got := ExtractImages(md); len(got) != 0 {
		t.Errorf("links must not match: %+v", got)
	}
}

func TestStripImages(t *testing.T) {
	t.Parallel()
	md := "Hello ![cat](https://x/y.png) world"
	got := StripImages(md)
	if strings.Contains(got, "!") || strings.Contains(got, "y.png") {
		t.Errorf("StripImages didn't remove image: %q", got)
	}
	if !strings.Contains(got, "Hello") || !strings.Contains(got, "world") {
		t.Errorf("StripImages removed surrounding text: %q", got)
	}
}
