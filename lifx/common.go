package lifx

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

// allWarmScene is the scene the lights return to after an effect.
const allWarmScene = "36e579f0-1160-4da3-92a3-e6e270f3cf48"

// startEffect starts the LIFX firmware effect name (e.g. "flame") on the
// lights matching selector, sending opts as the request body.
func startEffect(ctx context.Context, token, selector, name string, opts any) error {
	return sendToLIFX(ctx, http.MethodPost, token, lightsURL(selector, "effects/"+name), opts)
}

// setFromColour works around LIFX ignoring from_color on effects: it sets the
// bulbs to from first, so the effect starts from it as the "current" colour.
// A nil from leaves the bulbs as they are.
func setFromColour(ctx context.Context, token, selector string, from *Colour) error {
	if from == nil {
		return nil
	}
	if from.String() == "" {
		return errors.New("from colour is empty")
	}
	instant := 0.0
	state := LightState{Colour: *from, Duration: &instant}
	if err := sendToLIFX(ctx, http.MethodPut, token, lightsURL(selector, "state"), state); err != nil {
		return fmt.Errorf("setting from colour: %w", err)
	}
	return nil
}

// StopEffects stops any running firmware effect (pulse, breathe, flame,
// morph) on the lights matching selector, leaving them on.
func StopEffects(ctx context.Context, token, selector string) error {
	return startEffect(ctx, token, selector, "off", struct{}{})
}

// ResetLights stops any running effect on the lights matching selector, then
// activates the all-warm scene.
func ResetLights(ctx context.Context, token, selector string) error {
	if err := StopEffects(ctx, token, selector); err != nil {
		return fmt.Errorf("stopping effects: %w", err)
	}
	return activateAllWarm(ctx, token)
}

// activateAllWarm activates the all-warm scene. Use ResetLights instead
// unless the lights can't be running a firmware effect.
func activateAllWarm(ctx context.Context, token string) error {
	if err := ActivateScene(ctx, token, allWarmScene, SceneOptions{}); err != nil {
		return fmt.Errorf("activating scene: %w", err)
	}
	return nil
}

// Stop returns a handler that resets the lights matching the selector query
// parameter (default "all") with ResetLights, e.g.
// POST /lights/stop?selector=group:Living%20Room. Responds 204 on success or
// 502 if LIFX rejects either step.
func Stop(token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		selector := r.URL.Query().Get("selector")
		if selector == "" {
			selector = MakeSelector(All, "")
		}

		writeResult(w, ResetLights(r.Context(), token, selector))
	}
}
