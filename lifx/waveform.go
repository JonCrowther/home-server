package lifx

import (
	"context"
	"errors"
	"fmt"
)

// WaveformOptions are the settings for the pulse and breathe effects, which
// take the same options apart from Peak. It is also the JSON body sent to
// LIFX.
type WaveformOptions struct {
	// The colour to change to. Required.
	Colour Colour `json:"color"`
	// The colour to start the effect from. Nil uses the bulb's current colour.
	// Not sent to LIFX, which ignores from_color: the bulbs are set to it
	// first instead.
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
	// Breathe only: where in each cycle the colour is brightest, from 0.0 to
	// 1.0. Zero uses LIFX's default of 0.5, an even fade in and out.
	Peak float64 `json:"peak,omitempty"`
}

// setWaveform runs the waveform effect name ("pulse" or "breathe") on the
// lights matching selector, starting from opts.FromColour if it is set.
func setWaveform(ctx context.Context, token, selector, name string, opts WaveformOptions) error {
	if opts.Colour.String() == "" {
		return errors.New(name + " colour is required")
	}
	if err := setFromColour(ctx, token, selector, opts.FromColour); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return startEffect(ctx, token, selector, name, opts)
}
