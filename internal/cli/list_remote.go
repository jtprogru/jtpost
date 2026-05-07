package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jtprogru/jtpost/internal/adapters/apiclient"
	"github.com/spf13/cobra"
)

func contextBackground() context.Context { return context.Background() }

// runListRemote реализует `jtpost list --remote` через apiclient.ListPosts.
// F5b: proof-of-concept; full filter-coverage и table-вывод — F5c.
func runListRemote(cmd *cobra.Command, cli *apiclient.ClientWithResponses) error {
	params := &apiclient.ListPostsParams{}
	if listSearch != "" {
		params.Search = &listSearch
	}
	// status filter — берём первое значение (multi-status в spec не поддержан upstream).
	if len(listStatuses) > 0 {
		s := listStatuses[0]
		params.Status = &s
	}
	if len(listTags) > 0 {
		t := listTags[0]
		params.Tag = &t
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = cmd.Root().Context()
	}
	if ctx == nil {
		ctx = contextBackground()
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
	// F5b: simple JSON-вывод (table-mode для generated types — F5c).
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(posts)
}
