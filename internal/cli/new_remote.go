package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/jtprogru/jtpost/internal/adapters/apiclient"
)

// readContentSource читает контент из stdin (если path == "-"), файла или возвращает "" при пустом path.
func readContentSource(path string, stdin io.Reader) (string, error) {
	if path == "" {
		return "", nil
	}
	if path == "-" {
		b, err := io.ReadAll(stdin)
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		return string(b), nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read content file %q: %w", path, err)
	}
	return string(b), nil
}

// runNewRemote создаёт пост через POST /posts.
func runNewRemote(ctx context.Context, cli *apiclient.ClientWithResponses, title, slug, contentPath string, tags []string, stdin io.Reader, out io.Writer) error {
	if title == "" {
		return fmt.Errorf("--title required")
	}
	content, err := readContentSource(contentPath, stdin)
	if err != nil {
		return err
	}
	body := apiclient.CreatePostJSONRequestBody{Title: title}
	if slug != "" {
		body.Slug = &slug
	}
	if len(tags) > 0 {
		t := append([]string(nil), tags...)
		body.Tags = &t
	}
	if content != "" {
		body.Content = &content
	}
	resp, err := cli.CreatePostWithResponse(ctx, body)
	if err != nil {
		return fmt.Errorf("remote API call failed: %w", err)
	}
	switch resp.StatusCode() {
	case http.StatusCreated, http.StatusOK:
		// proceed
	case http.StatusBadRequest:
		return fmt.Errorf("validation error: %s", string(resp.Body))
	case http.StatusUnauthorized:
		return fmt.Errorf("unauthorized: invalid or expired --auth token")
	case http.StatusForbidden:
		return fmt.Errorf("forbidden")
	default:
		return fmt.Errorf("server error: %d", resp.StatusCode())
	}
	if resp.JSON201 == nil {
		fmt.Fprintln(out, "✅ Пост создан")
		return nil
	}
	fmt.Fprintf(out, "✅ Пост создан: %s\n", resp.JSON201.Title)
	fmt.Fprintf(out, "🆔 ID: %s\n", resp.JSON201.Id)
	fmt.Fprintf(out, "🏷️ Slug: %s\n", resp.JSON201.Slug)
	fmt.Fprintf(out, "📊 Статус: %s\n", resp.JSON201.Status)
	return nil
}
