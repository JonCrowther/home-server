package lifx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const (
	allWarmScene = "36e579f0-1160-4da3-92a3-e6e270f3cf48"
)

var (
	HabsRed = Colour{
		Hue:        354,
		Saturation: 0.83,
		Brightness: 0.69,
	}
	HabsBlue = Colour{
		Hue:        234,
		Saturation: 0.76,
		Brightness: 0.41,
	}
	OilersOrange = Colour{
		Hue:        18,
		Saturation: 1,
		Brightness: 1,
	}
	OilersBlue = Colour{
		Hue:        215,
		Saturation: 0.94,
		Brightness: 0.26,
	}
)

// putToLIFX sends payload as a PUT to endpoint and writes 204 on success or
// 502 with LIFX's status and body on failure.
func putToLIFX(w http.ResponseWriter, r *http.Request, token, endpoint string, payload any) {
	if err := sendToLIFX(r.Context(), http.MethodPut, token, endpoint, payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// sendToLIFX marshals payload as JSON and sends it to endpoint with the given
// method. It returns an error if the request fails or LIFX responds with a
// non-2xx status, including LIFX's status and body in the message.
func sendToLIFX(ctx context.Context, method, token, endpoint string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encoding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

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
	return nil
}

// setFromColour works around LIFX ignoring from_color on effects: it sets the
// bulbs to from first, so the effect starts from it as the "current" colour.
// A nil from leaves the bulbs as they are. escapedSelector must already be
// path-escaped.
func setFromColour(ctx context.Context, token, escapedSelector string, from *Colour) error {
	if from == nil {
		return nil
	}
	if from.String() == "" {
		return errors.New("from colour is empty")
	}
	instant := 0.0
	state := LightState{Colour: *from, Duration: &instant}
	endpoint := "https://api.lifx.com/v1/lights/" + escapedSelector + "/state"
	if err := sendToLIFX(ctx, http.MethodPut, token, endpoint, state); err != nil {
		return fmt.Errorf("setting from colour: %w", err)
	}
	return nil
}

// StopEffects stops any running firmware effect (pulse, breathe, flame,
// morph) on the lights matching selector, leaving them on.
func StopEffects(ctx context.Context, token, selector string) error {
	endpoint := "https://api.lifx.com/v1/lights/" + url.PathEscape(selector) + "/effects/off"
	return sendToLIFX(ctx, http.MethodPost, token, endpoint, struct{}{})
}

// Stop returns a handler that resets the lights: it stops any running effect
// on the lights matching the selector query parameter (default "all"), then
// activates the all-warm scene, e.g. POST /lights/stop?selector=group:Living%20Room.
// Responds 204 on success or 502 if either LIFX call fails.
func Stop(token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		selector := r.URL.Query().Get("selector")
		if selector == "" {
			selector = MakeSelector(All, "")
		}

		if err := StopEffects(r.Context(), token, selector); err != nil {
			http.Error(w, fmt.Sprintf("stopping effects: %v", err), http.StatusBadGateway)
			return
		}
		if err := ActivateScene(r.Context(), token, allWarmScene, SceneOptions{}); err != nil {
			http.Error(w, fmt.Sprintf("activating scene: %v", err), http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// Placeholder API function for testing light related code
func TestLights(token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pr := PulseOptions{
			Colour:     HabsRed,
			FromColour: &HabsBlue,
			Cycles:     5,
		}

		selector := MakeSelector(Group, "Living Room")

		SetPulse(r.Context(), token, selector, pr)
	}
}
