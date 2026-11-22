package lifx

import (
	"context"
	"errors"
	"net/http"
)

// MorphOptions are the settings for a morph effect, which slowly blends a
// palette of colours across the device. It is also the JSON body sent to
// LIFX. Like flame, morph is a firmware effect that only runs on matrix
// devices (LIFX Tile, Candle and similar).
type MorphOptions struct {
	// The time in seconds for the animation; lower is faster. Zero uses
	// LIFX's default of 5.
	Period float64 `json:"period,omitempty"`
	// How long in seconds to run the effect. Zero runs it until it is
	// stopped with StopEffects or another effect replaces it.
	Duration float64 `json:"duration,omitempty"`
	// The colours to blend between. Empty uses LIFX's default of seven
	// colours across the spectrum. Brightness is ignored: morph uses the
	// device's current brightness.
	Palette []Colour `json:"palette,omitempty"`
	// Whether to turn the bulb on if it is off. Nil uses LIFX's default of
	// true. A pointer so an explicit false is distinguishable from "not set".
	PowerOn *bool `json:"power_on,omitempty"`
}

// SetMorph starts a morph effect on the lights matching selector. Calling it
// again on running lights restarts the effect with the new options.
func SetMorph(ctx context.Context, token, selector string, opts MorphOptions) error {
	for _, c := range opts.Palette {
		if c.String() == "" {
			return errors.New("morph palette colour is empty")
		}
	}

	return startEffect(ctx, token, selector, "morph", opts)
}

// morphSunset is the default palette for Morph. Palette brightness is
// ignored, so only hue and saturation matter.
var morphSunset = []Colour{
	red,
	{Hue: 25, Saturation: 1},  // orange
	{Hue: 45, Saturation: 1},  // amber
	{Hue: 330, Saturation: 1}, // pink
	{Hue: 280, Saturation: 1}, // purple
}

// Morph returns a handler that starts a morph effect on the lights in the
// {selector} path value, e.g.
// POST /lights/{selector}/morph?period=3&palette=FF0000,FFA500,800080.
//
// Query parameters, all optional: period (default 1), duration, palette
// (comma-separated hex, default a sunset palette) and power_on. Responds 204
// once started, 400 for a bad parameter, or 502 if LIFX rejects it.
func Morph(token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		opts := MorphOptions{
			Period:  1,
			Palette: morphSunset,
		}

		q := newQueryParser(r)
		q.float("period", &opts.Period)
		q.float("duration", &opts.Duration)
		q.colours("palette", &opts.Palette)
		q.optionalBool("power_on", &opts.PowerOn)
		if q.err != nil {
			http.Error(w, q.err.Error(), http.StatusBadRequest)
			return
		}

		writeResult(w, r, SetMorph(r.Context(), token, r.PathValue("selector"), opts))
	}
}
