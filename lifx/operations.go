package lifx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
)

// LoadLights returns a handler that lists all lights as JSON.
func LoadLights(token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		url := "https://api.lifx.com/v1/lights"

		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, url, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if resp.StatusCode != http.StatusOK {
			http.Error(w, fmt.Sprintf("%s: %s", resp.Status, body), http.StatusBadGateway)
			return
		}

		var maps []map[string]any
		if err := json.Unmarshal(body, &maps); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		lights := make([]Light, 0, len(maps))
		for _, m := range maps {
			lights = append(lights, ToLight(m))
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(lights); err != nil {
			slog.Error("encoding lights response", "err", err)
		}
	}
}

// GetScenes returns a handler that lists all scenes as JSON.
func GetScenes(token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		url := "https://api.lifx.com/v1/scenes"

		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, url, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if resp.StatusCode != http.StatusOK {
			http.Error(w, fmt.Sprintf("%s: %s", resp.Status, body), http.StatusBadGateway)
			return
		}

		var scenes []Scene
		if err := json.Unmarshal(body, &scenes); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(scenes); err != nil {
			slog.Error("encoding scenes response", "err", err)
		}
	}
}

// SetScene returns a handler that activates the scene in the {uuid} path
// value, e.g. PUT /scenes/{uuid}/activate. The JSON body is optional and
// takes SceneOptions.
func SetScene(token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		endpoint := "https://api.lifx.com/v1/scenes/scene_id:" + url.PathEscape(uuid) + "/activate"

		var opts SceneOptions
		if err := json.NewDecoder(r.Body).Decode(&opts); err != nil && err != io.EOF {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		putToLIFX(w, r, token, endpoint, opts)
	}
}

// SetLight returns a handler that applies a LightState (JSON body) to the
// selector in the {selector} path value, e.g. PUT /lights/{selector}/state.
func SetLight(token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		selector := r.PathValue("selector")
		endpoint := "https://api.lifx.com/v1/lights/" + url.PathEscape(selector) + "/state"

		var state LightState
		if err := json.NewDecoder(r.Body).Decode(&state); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		putToLIFX(w, r, token, endpoint, state)
	}
}

// SetMultipleLights returns a handler that applies several states at once.
// The JSON body mirrors LIFX's own: {"states": [{"selector": "...", ...}]}.
func SetMultipleLights(token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			States []SelectorState `json:"states"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		putToLIFX(w, r, token, "https://api.lifx.com/v1/lights/states", payload)
	}
}

// putToLIFX sends payload as a PUT to endpoint and writes 204 on success or
// 502 with LIFX's status and body on failure.
func putToLIFX(w http.ResponseWriter, r *http.Request, token, endpoint string, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if resp.StatusCode >= http.StatusMultipleChoices {
		http.Error(w, fmt.Sprintf("%s: %s", resp.Status, respBody), http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
