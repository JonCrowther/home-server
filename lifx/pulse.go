package lifx

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// PulseOptions are the settings for a pulse effect. It is also the JSON body
// sent to LIFX.
type PulseOptions struct {
	// The colour to pulse to. Required.
	Colour Colour `json:"color"`
	// The colour to start the effect from. Nil uses the bulb's current colour.
	// Not sent to LIFX, which ignores from_color: SetPulse sets the bulbs to
	// it first instead.
	FromColour *Colour `json:"-"`
	// The time in seconds for one cycle of the effect. Zero uses LIFX's
	// default of 1.
	Period float64 `json:"period,omitempty"`
	// The number of times to repeat the effect. Zero uses LIFX's default of 1.
	Cycles float64 `json:"cycles,omitempty"`
	// If true, leave the light on the last effect colour when the effect ends.
	// If false, restore the light to its previous state.
	Persist bool `json:"persist,omitempty"`
	// Whether to turn the bulb on if it is off. Nil uses LIFX's default of
	// true. A pointer so an explicit false is distinguishable from "not set".
	PowerOn *bool `json:"power_on,omitempty"`
}

// SetPulse runs a pulse effect on the lights matching selector, switching
// between FromColour (or the current colour) and Colour.
func SetPulse(ctx context.Context, token, selector string, opts PulseOptions) error {
	if opts.Colour.String() == "" {
		return errors.New("pulse colour is required")
	}

	escaped := url.PathEscape(selector)

	if err := setFromColour(ctx, token, escaped, opts.FromColour); err != nil {
		return fmt.Errorf("pulse: %w", err)
	}

	endpoint := "https://api.lifx.com/v1/lights/" + escaped + "/effects/pulse"
	return sendToLIFX(ctx, http.MethodPost, token, endpoint, opts)
}

// CelebrateGoal pulses the lights matching selector between from and to,
// waits for the pulse to finish, then activates the all-warm scene. It blocks
// for the length of the pulse.
func CelebrateGoal(ctx context.Context, token, selector string, from, to Colour) error {
	opts := PulseOptions{
		Colour:     to,
		FromColour: &from,
		Period:     0.5,
		Cycles:     15,
	}
	if err := SetPulse(ctx, token, selector, opts); err != nil {
		return fmt.Errorf("pulsing: %w", err)
	}

	// The pulse API returns as soon as the effect starts. Activating the
	// scene straight away would cut the pulse off, so wait for it to end.
	wait := time.Duration(opts.Period * opts.Cycles * float64(time.Second))
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(wait):
	}

	if err := ActivateScene(ctx, token, allWarmScene, SceneOptions{}); err != nil {
		return fmt.Errorf("activating scene: %w", err)
	}
	return nil
}
