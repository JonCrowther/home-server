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
	"strings"
	"time"
)

// apiBase is the root of the LIFX HTTP API.
const apiBase = "https://api.lifx.com/v1"

// httpClient sends every LIFX request. The timeout stops a hung request from
// blocking its caller (e.g. a goal celebration) forever.
var httpClient = &http.Client{Timeout: 15 * time.Second}

// lightsURL returns the LIFX endpoint for path on the lights matching
// selector, escaping the selector, e.g. lightsURL("group:Living Room",
// "effects/flame").
func lightsURL(selector, path string) string {
	return apiBase + "/lights/" + url.PathEscape(selector) + "/" + path
}

// sendToLIFX marshals payload as JSON and sends it to endpoint with the given
// method. It returns an error if the request fails, LIFX responds with a
// non-2xx status, or none of the lights it targeted responded.
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
// as an error with LIFX's status and body, and a 207 Multi-Status is checked
// light by light with checkResults. If dst is non-nil, the response body is
// decoded into it.
func doLIFX(ctx context.Context, method, token, endpoint string, body io.Reader, dst any) error {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	start := time.Now()
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, endpoint, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("%s %s: reading response: %w", method, endpoint, err)
	}
	slog.Debug("LIFX request", "method", method, "endpoint", endpoint,
		"status", resp.StatusCode, "duration", time.Since(start))

	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return fmt.Errorf("%s %s: rate limited by LIFX (120 requests a minute), resets at %s",
			method, endpoint, resp.Header.Get("X-RateLimit-Reset"))
	case resp.StatusCode >= http.StatusMultipleChoices:
		return fmt.Errorf("%s %s: %s: %s", method, endpoint, resp.Status, bytes.TrimSpace(respBody))
	case resp.StatusCode == http.StatusMultiStatus:
		if err := checkResults(endpoint, respBody); err != nil {
			return fmt.Errorf("%s %s: %w", method, endpoint, err)
		}
	}

	if dst != nil {
		if err := json.Unmarshal(respBody, dst); err != nil {
			return fmt.Errorf("%s %s: decoding response: %w", method, endpoint, err)
		}
	}
	return nil
}

// lightResult is one light's outcome in a LIFX 207 Multi-Status response,
// e.g. {"label": "Left Lamp Upper", "status": "timed_out"}. PUT /lights/states
// nests a list of these under each operation, held in Results.
type lightResult struct {
	ID      string        `json:"id"`
	Label   string        `json:"label"`
	Status  string        `json:"status"`
	Results []lightResult `json:"results"`
}

// checkResults checks the per-light results LIFX sends with a 207. LIFX uses
// 207 even when lights didn't respond ("timed_out" or "offline"), so a 2xx
// status alone doesn't mean anything happened. It returns an error if no
// light responded, and logs a warning naming the lights that didn't if only
// some did.
func checkResults(endpoint string, body []byte) error {
	var resp struct {
		Results []lightResult `json:"results"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("decoding light results: %w", err)
	}

	ok := 0
	var failed []string
	var walk func([]lightResult)
	walk = func(results []lightResult) {
		for _, r := range results {
			switch {
			case len(r.Results) > 0:
				walk(r.Results)
			case r.Status == "ok":
				ok++
			case r.Status != "":
				name := r.Label
				if name == "" {
					name = r.ID
				}
				failed = append(failed, name+": "+r.Status)
			}
		}
	}
	walk(resp.Results)

	if len(failed) == 0 {
		return nil
	}
	if ok == 0 {
		return fmt.Errorf("no lights responded (%s)", strings.Join(failed, ", "))
	}
	slog.Warn("some lights didn't respond", "endpoint", endpoint, "ok", ok, "failed", failed)
	return nil
}

// putToLIFX sends payload as a PUT to endpoint and writes the result with
// writeResult.
func putToLIFX(w http.ResponseWriter, r *http.Request, token, endpoint string, payload any) {
	writeResult(w, r, sendToLIFX(r.Context(), http.MethodPut, token, endpoint, payload))
}

// writeResult writes the response for a handler whose work was a call to
// LIFX: 502 with the error if err is non-nil (see writeLIFXError), otherwise
// 204.
func writeResult(w http.ResponseWriter, r *http.Request, err error) {
	if err != nil {
		writeLIFXError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeLIFXError logs a failed call to LIFX made for request r and responds
// 502 with the error.
func writeLIFXError(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("LIFX call failed", "method", r.Method, "path", r.URL.Path, "err", err)
	http.Error(w, err.Error(), http.StatusBadGateway)
}

// writeJSON writes v as a JSON response.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encoding response", "err", err)
	}
}
