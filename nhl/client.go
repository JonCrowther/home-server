package nhl

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// nhleAPI is the root of the NHL's public web API.
const nhleAPI = "https://api-web.nhle.com/v1"

// getJSON sends a GET to endpoint and decodes the JSON response into dst.
func getJSON(ctx context.Context, endpoint string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: unexpected status %s", endpoint, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("decoding %s: %w", endpoint, err)
	}
	return nil
}
