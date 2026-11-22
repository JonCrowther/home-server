package lifx

import (
	"context"
	"net/http"
)

// SetBreathe runs a breathe effect on the lights matching selector, fading
// smoothly between FromColour (or the current colour) and Colour. It returns
// as soon as the effect starts.
func SetBreathe(ctx context.Context, token, selector string, opts WaveformOptions) error {
	return setWaveform(ctx, token, selector, "breathe", opts)
}

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
		from := blue
		opts := WaveformOptions{
			Colour:     red,
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

		writeResult(w, r, SetBreathe(r.Context(), token, r.PathValue("selector"), opts))
	}
}
