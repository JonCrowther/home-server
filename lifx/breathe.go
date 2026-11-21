package lifx

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

// BreatheOptions are the settings for a breathe effect. It is also the JSON
// body sent to LIFX. Unlike a pulse, which switches sharply between colours,
// a breathe fades smoothly between them.
type BreatheOptions struct {
	// The colour to breathe to. Required.
	Colour Colour `json:"color"`
	// The colour to start the effect from. Nil uses the bulb's current colour.
	// Not sent to LIFX, which ignores from_color: SetBreathe sets the bulbs to
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
	// Where in each cycle the colour is brightest, from 0.0 to 1.0. Zero uses
	// LIFX's default of 0.5, an even fade in and out.
	Peak float64 `json:"peak,omitempty"`
}

// SetBreathe runs a breathe effect on the lights matching selector, fading
// between FromColour (or the current colour) and Colour. It returns as soon
// as the effect starts.
func SetBreathe(ctx context.Context, token, selector string, opts BreatheOptions) error {
	if opts.Colour.String() == "" {
		return errors.New("breathe colour is required")
	}

	escaped := url.PathEscape(selector)

	if err := setFromColour(ctx, token, escaped, opts.FromColour); err != nil {
		return fmt.Errorf("breathe: %w", err)
	}

	endpoint := "https://api.lifx.com/v1/lights/" + escaped + "/effects/breathe"
	return sendToLIFX(ctx, http.MethodPost, token, endpoint, opts)
}

// breatheBlue and breatheRed are Breathe's default colours. Red uses a hue of
// 360 rather than 0, because Colour.String drops zero fields and a missing
// hue would leave the bulb on its current hue.
var (
	breatheBlue = Colour{Hue: 240, Saturation: 1, Brightness: 1}
	breatheRed  = Colour{Hue: 360, Saturation: 1, Brightness: 1}
)

// Breathe returns a handler that runs a breathe effect on the lights in the
// {selector} path value, e.g.
// POST /lights/{selector}/breathe?colour=F47A38&period=2.
//
// Query parameters, all optional: colour and from_colour (hex, default red
// and blue), period (default 4), cycles (default 5), persist, power_on and
// peak. Responds 204 once started, 400 for a bad parameter, or 502 if LIFX
// rejects it.
func Breathe(token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Copied so parsing from_colour can't overwrite the package default.
		from := breatheBlue
		opts := BreatheOptions{
			Colour:     breatheRed,
			FromColour: &from,
			Period:     4,
			Cycles:     5,
		}

		q := newQueryParser(r)
		q.colour("colour", &opts.Colour)
		q.colour("from_colour", &from)
		q.float("period", &opts.Period)
		q.float("cycles", &opts.Cycles)
		q.bool("persist", &opts.Persist)
		q.optionalBool("power_on", &opts.PowerOn)
		q.float("peak", &opts.Peak)
		if q.err != nil {
			http.Error(w, q.err.Error(), http.StatusBadRequest)
			return
		}

		if err := SetBreathe(r.Context(), token, r.PathValue("selector"), opts); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
