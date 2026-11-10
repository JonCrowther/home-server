package lifx

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

// PulseOptions are the settings for a pulse effect.
type PulseOptions struct {
	// The colour to pulse to. Required.
	Colour Colour
	// The colour to start the effect from. Nil uses the bulb's current colour.
	FromColour *Colour
	// The time in seconds for one cycle of the effect. Zero uses LIFX's
	// default of 1.
	Period float64
	// The number of times to repeat the effect. Zero uses LIFX's default of 1.
	Cycles float64
	// If true, leave the light on the last effect colour when the effect ends.
	// If false, restore the light to its previous state.
	Persist bool
	// Whether to turn the bulb on if it is off. Nil uses LIFX's default of
	// true. A pointer so an explicit false is distinguishable from "not set".
	PowerOn *bool
}

// pulseRequest is the JSON body LIFX expects, with colours as strings.
//
// LIFX's from_color field is deliberately absent: the API ignores it and
// always pulses from the bulb's current colour. SetPulse sets the current
// colour to FromColour first instead.
type pulseRequest struct {
	Colour  string  `json:"color"`
	Period  float64 `json:"period,omitempty"`
	Cycles  float64 `json:"cycles,omitempty"`
	Persist bool    `json:"persist,omitempty"`
	PowerOn *bool   `json:"power_on,omitempty"`
}

// fromColourRequest is the state body used to set the starting colour before
// a pulse. Duration has no omitempty because LIFX treats a missing duration
// as 1 second, and we want the change to be instant.
type fromColourRequest struct {
	Colour   string  `json:"color"`
	Duration float64 `json:"duration"`
}

// SetPulse runs a pulse effect on the lights matching selector, switching
// between FromColour (or the current colour) and Colour.
func SetPulse(ctx context.Context, token, selector string, opts PulseOptions) error {
	colour := opts.Colour.String()
	if colour == "" {
		return errors.New("pulse colour is required")
	}

	escaped := url.PathEscape(selector)

	// Work around LIFX ignoring from_color: set the bulbs to FromColour
	// first, so the pulse starts from it as the "current" colour.
	if opts.FromColour != nil {
		from := opts.FromColour.String()
		if from == "" {
			return errors.New("pulse from colour is empty")
		}
		stateEndpoint := "https://api.lifx.com/v1/lights/" + escaped + "/state"
		if err := sendToLIFX(ctx, http.MethodPut, token, stateEndpoint, fromColourRequest{Colour: from}); err != nil {
			return fmt.Errorf("setting pulse from colour: %w", err)
		}
	}

	payload := pulseRequest{
		Colour:  colour,
		Period:  opts.Period,
		Cycles:  opts.Cycles,
		Persist: opts.Persist,
		PowerOn: opts.PowerOn,
	}

	endpoint := "https://api.lifx.com/v1/lights/" + escaped + "/effects/pulse"
	return sendToLIFX(ctx, http.MethodPost, token, endpoint, payload)
}
