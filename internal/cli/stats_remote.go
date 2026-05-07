package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/jtprogru/jtpost/internal/adapters/apiclient"
)

func runStatsRemote(ctx context.Context, cli *apiclient.ClientWithResponses, out io.Writer) error {
	resp, err := cli.GetStatsWithResponse(ctx)
	if err != nil {
		return fmt.Errorf("remote API call failed: %w", err)
	}
	switch resp.StatusCode() {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return fmt.Errorf("unauthorized: invalid or expired --auth token")
	default:
		return fmt.Errorf("remote API returned %d", resp.StatusCode())
	}
	if resp.JSON200 == nil {
		return fmt.Errorf("empty response body")
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(*resp.JSON200)
}
