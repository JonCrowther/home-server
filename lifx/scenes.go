package lifx

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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
		var scenes []Scene
		if err := getFromLIFX(r.Context(), token, apiBase+"/scenes", &scenes); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, scenes)
	}
}

// SetScene returns a handler that activates the scene in the {uuid} path
// value, e.g. PUT /scenes/{uuid}/activate. The JSON body is optional and
// takes SceneOptions.
func SetScene(token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var opts SceneOptions
		if err := json.NewDecoder(r.Body).Decode(&opts); err != nil && !errors.Is(err, io.EOF) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		writeResult(w, ActivateScene(r.Context(), token, r.PathValue("uuid"), opts))
	}
}

// ActivateScene activates the scene with the given UUID.
func ActivateScene(ctx context.Context, token, uuid string, opts SceneOptions) error {
	endpoint := apiBase + "/scenes/scene_id:" + url.PathEscape(uuid) + "/activate"
	return sendToLIFX(ctx, http.MethodPut, token, endpoint, opts)
}
