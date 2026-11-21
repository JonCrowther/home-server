package lifx

import (
	"encoding/json"
	"net/http"
)

// LightState is a state to set on lights, and the state reported for a light
// by LoadLights.
type LightState struct {
	// The power state you want to set on the selector. on or off
	Power string `json:"power,omitempty"`
	// The colour to set the light to. Left out if zero.
	Colour Colour `json:"color,omitzero"`
	// The brightness level from 0.0 to 1.0. Overrides any brightness set in color (if any).
	Brightness float64 `json:"brightness,omitempty"`
	// How long in seconds you want the power action to take. Range: 0.0 – 3155760000.0 (100 years)
	// A pointer so an explicit 0 (instant) is distinguishable from "not set",
	// which LIFX treats as 1.
	Duration *float64 `json:"duration,omitempty"`
	// The maximum brightness of the infrared channel from 0.0 to 1.0.
	Infrared float64 `json:"infrared,omitempty"`
	// Execute the query fast, without initial state checks and wait for no results.
	Fast bool `json:"fast,omitempty"`
}

// SelectorState pairs a selector with the state to set on it.
type SelectorState struct {
	Selector string `json:"selector"`
	LightState
}

// statesRequest is the body of LIFX's PUT /lights/states, which sets several
// selectors at once.
type statesRequest struct {
	States []SelectorState `json:"states"`
}

// Light is a trimmed-down LIFX light, as returned by LoadLights.
type Light struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	GroupID      string     `json:"group_id"`
	GroupName    string     `json:"group_name"`
	LocationID   string     `json:"location_id"`
	LocationName string     `json:"location_name"`
	MaxKelvin    int        `json:"max_kelvin"`
	MinKelvin    int        `json:"min_kelvin"`
	State        LightState `json:"light_state"`
}

// lifxLight is the part of a light in LIFX's GET /lights response that Light
// keeps. Decoding into it ignores everything else LIFX sends.
type lifxLight struct {
	ID         string  `json:"id"`
	Label      string  `json:"label"`
	Power      string  `json:"power"`
	Brightness float64 `json:"brightness"`
	Colour     Colour  `json:"color"`
	Group      struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"group"`
	Location struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"location"`
	Product struct {
		Capabilities struct {
			MinKelvin int `json:"min_kelvin"`
			MaxKelvin int `json:"max_kelvin"`
		} `json:"capabilities"`
	} `json:"product"`
}

// toLight flattens a LIFX light into a Light.
func (l lifxLight) toLight() Light {
	return Light{
		ID:           l.ID,
		Name:         l.Label,
		GroupID:      l.Group.ID,
		GroupName:    l.Group.Name,
		LocationID:   l.Location.ID,
		LocationName: l.Location.Name,
		MaxKelvin:    l.Product.Capabilities.MaxKelvin,
		MinKelvin:    l.Product.Capabilities.MinKelvin,
		State: LightState{
			Power:      l.Power,
			Brightness: l.Brightness,
			Colour:     l.Colour,
		},
	}
}

// LoadLights returns a handler that lists all lights as JSON.
func LoadLights(token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var raw []lifxLight
		if err := getFromLIFX(r.Context(), token, apiBase+"/lights", &raw); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		lights := make([]Light, 0, len(raw))
		for _, l := range raw {
			lights = append(lights, l.toLight())
		}
		writeJSON(w, lights)
	}
}

// SetLight returns a handler that applies a LightState (JSON body) to the
// selector in the {selector} path value, e.g. PUT /lights/{selector}/state.
func SetLight(token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var state LightState
		if err := json.NewDecoder(r.Body).Decode(&state); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		putToLIFX(w, r, token, lightsURL(r.PathValue("selector"), "state"), state)
	}
}

// SetMultipleLights returns a handler that applies several states at once.
// The JSON body mirrors LIFX's own: {"states": [{"selector": "...", ...}]}.
func SetMultipleLights(token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload statesRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		putToLIFX(w, r, token, apiBase+"/lights/states", payload)
	}
}
