package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/jtprogru/jtpost/internal/adapters/apiclient"
)

// runListRemote реализует `jtpost list --remote` через apiclient.ListPosts.
// Принимает уже подготовленный ctx и client из runRemote helper.
func runListRemote(ctx context.Context, cli *apiclient.ClientWithResponses, out io.Writer) error {
	params := &apiclient.ListPostsParams{}
	if listSearch != "" {
		params.Search = &listSearch
	}
	if len(listStatuses) > 0 {
		s := listStatuses[0]
		params.Status = &s
	}
	if len(listTags) > 0 {
		t := listTags[0]
		params.Tag = &t
	}

	resp, err := cli.ListPostsWithResponse(ctx, params)
	if err != nil {
		return fmt.Errorf("remote API call failed: %w", err)
	}
	switch resp.StatusCode() {
	case http.StatusOK:
		// proceed
	case http.StatusUnauthorized:
		return fmt.Errorf("unauthorized: invalid or expired --auth token")
	default:
		return fmt.Errorf("remote API returned %d", resp.StatusCode())
	}

	posts := []apiclient.Post{}
	if resp.JSON200 != nil {
		posts = *resp.JSON200
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(posts)
}
