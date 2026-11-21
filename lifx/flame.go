package lifx

import (
	"context"
	"net/http"
	"net/url"
)

// FlameOptions are the settings for a flame effect. It is also the JSON body
// sent to LIFX. Flame is a firmware effect that only runs on matrix devices
// (LIFX Tile, Candle and similar); other bulbs ignore it.
type FlameOptions struct {
	// The time in seconds for one cycle of the flicker; lower is faster.
	// Zero uses LIFX's default of 5.
	Period float64 `json:"period,omitempty"`
	// How long in seconds to run the effect. Zero runs it until it is
	// stopped with StopEffects or another effect replaces it.
	Duration float64 `json:"duration,omitempty"`
	// Whether to turn the bulb on if it is off. Nil uses LIFX's default of
	// true. A pointer so an explicit false is distinguishable from "not set".
	PowerOn *bool `json:"power_on,omitempty"`
}

// SetFlame starts a flame effect on the lights matching selector. Calling it
// again on running lights restarts the effect with the new options.
func SetFlame(ctx context.Context, token, selector string, opts FlameOptions) error {
	endpoint := "https://api.lifx.com/v1/lights/" + url.PathEscape(selector) + "/effects/flame"
	return sendToLIFX(ctx, http.MethodPost, token, endpoint, opts)
}

// Flame returns a handler that starts a flame effect on the lights in the
// {selector} path value, e.g. POST /lights/{selector}/flames?period=3.
//
// Query parameters, all optional: period (lower flickers faster), duration
// and power_on. Responds 204 once started, 400 for a bad parameter, or 502
// if LIFX rejects it.
func Flame(token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var opts FlameOptions

		q := newQueryParser(r)
		q.float("period", &opts.Period)
		q.float("duration", &opts.Duration)
		q.optionalBool("power_on", &opts.PowerOn)
		if q.err != nil {
			http.Error(w, q.err.Error(), http.StatusBadRequest)
			return
		}

		if err := SetFlame(r.Context(), token, r.PathValue("selector"), opts); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
