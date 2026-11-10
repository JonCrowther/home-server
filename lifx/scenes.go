package lifx

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
)

// Scene is a trimmed-down LIFX scene. The per-light colour data LIFX returns
// (the "chain" of zone colours) is dropped, since it is large and not needed
// to list or activate a scene.
type Scene struct {
	UUID   string       `json:"uuid"`
	Name   string       `json:"name"`
	States []SceneState `json:"states"`
}

// SceneState is the light a scene applies to and the power it sets.
type SceneState struct {
	Selector string `json:"selector"`
	Power    string `json:"power"`
}

// SceneOptions are the optional settings when activating a scene.
type SceneOptions struct {
	// Time in seconds to transition from the current light state to the scene
	// state. A pointer so an explicit 0 (instant) is distinguishable from
	// "not set", which LIFX treats as 1.
	Duration *float64 `json:"duration,omitempty"`
	// Properties that should not be changed on devices when applying the
	// scene. Any of "power", "infrared", "duration", "intensity", "hue",
	// "saturation", "brightness" or "kelvin".
	Ignore []string `json:"ignore,omitempty"`
	// Execute the query fast, without initial state checks and wait for no results.
	Fast bool `json:"fast,omitempty"`
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
		var opts SceneOptions
		if err := json.NewDecoder(r.Body).Decode(&opts); err != nil && err != io.EOF {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := ActivateScene(r.Context(), token, r.PathValue("uuid"), opts); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ActivateScene activates the scene with the given UUID.
func ActivateScene(ctx context.Context, token, uuid string, opts SceneOptions) error {
	endpoint := "https://api.lifx.com/v1/scenes/scene_id:" + url.PathEscape(uuid) + "/activate"
	return sendToLIFX(ctx, http.MethodPut, token, endpoint, opts)
}
