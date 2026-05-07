package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/jtprogru/jtpost/internal/adapters/apiclient"
)

func runPlanRemote(ctx context.Context, cli *apiclient.ClientWithResponses, out io.Writer) error {
	resp, err := cli.GetPlanWithResponse(ctx, &apiclient.GetPlanParams{})
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
	plan := []apiclient.PlanItem{}
	if resp.JSON200 != nil {
		plan = *resp.JSON200
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(plan)
}
