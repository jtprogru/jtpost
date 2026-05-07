package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/adapters/apiclient"
)

// runPublishRemote реализует `jtpost publish <id> --remote ...` через apiclient.
func runPublishRemote(ctx context.Context, cli *apiclient.ClientWithResponses, idArg string, out io.Writer) error {
	id, err := uuid.Parse(idArg)
	if err != nil {
		return fmt.Errorf("--remote requires UUID id: %w", err)
	}
	resp, err := cli.PublishPostWithResponse(ctx, id)
	if err != nil {
		return fmt.Errorf("remote API call failed: %w", err)
	}
	switch resp.StatusCode() {
	case http.StatusOK:
		// proceed
	case http.StatusBadRequest:
		return fmt.Errorf("publish rejected: %s", string(resp.Body))
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
	if resp.JSON200 == nil {
		fmt.Fprintf(out, "✅ Пост опубликован: %s\n", id)
		return nil
	}
	fmt.Fprintf(out, "✅ Пост опубликован: %s\n", id)
	if resp.JSON200.Title != "" {
		fmt.Fprintf(out, "📝 Заголовок: %s\n", resp.JSON200.Title)
	}
	return nil
}
