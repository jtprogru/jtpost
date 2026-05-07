package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/adapters/apiclient"
)

// runEditRemote применяет partial update через PATCH /posts/{id}.
// hasTags / hasStatus отличают "не задано" от "пустое".
func runEditRemote(ctx context.Context, cli *apiclient.ClientWithResponses, idArg, title, contentPath, status string, tags []string, hasTags bool, stdin io.Reader, out io.Writer) error {
	id, err := uuid.Parse(idArg)
	if err != nil {
		return fmt.Errorf("--remote requires UUID id: %w", err)
	}
	body := apiclient.UpdatePostJSONRequestBody{}
	dirty := false
	if title != "" {
		body.Title = &title
		dirty = true
	}
	if contentPath != "" {
		content, err := readContentSource(contentPath, stdin)
		if err != nil {
			return err
		}
		body.Content = &content
		dirty = true
	}
	if hasTags {
		t := append([]string(nil), tags...)
		body.Tags = &t
		dirty = true
	}
	if status != "" {
		s := apiclient.PostStatus(status)
		body.Status = &s
		dirty = true
	}
	if !dirty {
		return fmt.Errorf("no fields to update: provide --title, --content, --tag, or --status")
	}
	resp, err := cli.UpdatePostWithResponse(ctx, id, body)
	if err != nil {
		return fmt.Errorf("remote API call failed: %w", err)
	}
	switch resp.StatusCode() {
	case http.StatusOK:
		// proceed
	case http.StatusBadRequest:
		return fmt.Errorf("validation error: %s", string(resp.Body))
	case http.StatusUnauthorized:
		return fmt.Errorf("unauthorized: invalid or expired --auth token")
	case http.StatusForbidden:
		return fmt.Errorf("forbidden")
	case http.StatusNotFound:
		return fmt.Errorf("post not found: %s", id)
	case http.StatusConflict:
		return fmt.Errorf("conflict: %s", string(resp.Body))
	default:
		return fmt.Errorf("server error: %d", resp.StatusCode())
	}
	if resp.JSON200 != nil {
		fmt.Fprintf(out, "✅ Пост обновлён: %s\n", resp.JSON200.Title)
		fmt.Fprintf(out, "📊 Статус: %s\n", resp.JSON200.Status)
		fmt.Fprintf(out, "🔢 Revision: %d\n", resp.JSON200.Revision)
	} else {
		fmt.Fprintf(out, "✅ Пост обновлён: %s\n", id)
	}
	return nil
}
