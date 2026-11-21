package lifx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
)

// apiBase is the root of the LIFX HTTP API.
const apiBase = "https://api.lifx.com/v1"

// lightsURL returns the LIFX endpoint for path on the lights matching
// selector, escaping the selector, e.g. lightsURL("group:Living Room",
// "effects/flame").
func lightsURL(selector, path string) string {
	return apiBase + "/lights/" + url.PathEscape(selector) + "/" + path
}

// sendToLIFX marshals payload as JSON and sends it to endpoint with the given
// method. It returns an error if the request fails or LIFX responds with a
// non-2xx status, including LIFX's status and body in the message.
func sendToLIFX(ctx context.Context, method, token, endpoint string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encoding request: %w", err)
	}
	return doLIFX(ctx, method, token, endpoint, bytes.NewReader(body), nil)
}

// getFromLIFX sends a GET to endpoint and decodes the JSON response into dst.
func getFromLIFX(ctx context.Context, token, endpoint string, dst any) error {
	return doLIFX(ctx, http.MethodGet, token, endpoint, nil, dst)
}

// doLIFX sends a request to LIFX with the token. A non-2xx status is returned
// as an error with LIFX's status and body. If dst is non-nil, the response
// body is decoded into it.
func doLIFX(ctx context.Context, method, token, endpoint string, body io.Reader, dst any) error {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%s: %s", resp.Status, respBody)
	}

	if dst != nil {
		if err := json.Unmarshal(respBody, dst); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}
	return nil
}

// putToLIFX sends payload as a PUT to endpoint and writes the result with
// writeResult.
func putToLIFX(w http.ResponseWriter, r *http.Request, token, endpoint string, payload any) {
	writeResult(w, sendToLIFX(r.Context(), http.MethodPut, token, endpoint, payload))
}

// writeResult writes the response for a handler whose work was a call to
// LIFX: 502 with the error if err is non-nil, otherwise 204.
func writeResult(w http.ResponseWriter, err error) {
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeJSON writes v as a JSON response.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encoding response", "err", err)
	}
}
