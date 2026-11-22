package lifx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

// sweepLights are the labels of the lights the sweep moves across, in order
// from left to right. They must match the labels set in the LIFX app.
var sweepLights = []string{
	"Left Lamp Upper",
	"Left Lamp Lower",
	"Right Lamp Lower",
	"Right Lamp Upper",
}

// SweepOptions are the settings for a sweep effect.
type SweepOptions struct {
	// Labels of the lights to sweep across, in order. At least two.
	Lights []string
	// The colour that moves along the lights.
	Colour Colour
	// The colour of every other light.
	Background Colour
	// How long the colour stays on each light, including the fade onto it.
	Step time.Duration
	// The number of times to go there and back.
	Passes int
}

// sweepPositions returns the index of the lit light for each step: along the
// lights and back, passes times, ending where it started. For 4 lights and 1
// pass: 0 1 2 3 2 1 0.
func sweepPositions(lights, passes int) []int {
	positions := []int{0}
	for range passes {
		for i := 1; i < lights; i++ {
			positions = append(positions, i)
		}
		for i := lights - 2; i >= 0; i-- {
			positions = append(positions, i)
		}
	}
	return positions
}

// sweepFrame is the request for one step of a sweep: the light at index lit
// gets colour and every other light gets background, each fading over fade
// seconds.
func sweepFrame(lights []string, lit int, colour, background Colour, fade float64) statesRequest {
	req := statesRequest{States: make([]SelectorState, len(lights))}
	for i, label := range lights {
		c := background
		if i == lit {
			c = colour
		}
		req.States[i] = SelectorState{
			Selector: MakeSelector(Label, label),
			LightState: LightState{
				Colour:   c,
				Duration: &fade,
				// Don't wait for LIFX to confirm each bulb; keeps steps on time.
				Fast: true,
			},
		}
	}
	return req
}

// RunSweep moves opts.Colour along opts.Lights and back, with every other
// light set to opts.Background. Each step fades the old light to the
// background as the next light fades in. It blocks until the sweep ends,
// then activates the all-warm scene. There's no firmware effect to stop, so
// it skips the stop step of ResetLights.
//
// Each step is one request, so a sweep sends 2×(lights-1)×passes+1 requests
// plus one for the scene: 20 for four lights and three passes. LIFX allows
// 120 a minute, so a single sweep is fine at any step, but many sweeps in a
// row at a short step can run into the limit.
func RunSweep(ctx context.Context, token string, opts SweepOptions) error {
	if len(opts.Lights) < 2 {
		return errors.New("sweep needs at least two lights")
	}
	if opts.Colour.String() == "" || opts.Background.String() == "" {
		return errors.New("sweep colour and background are required")
	}
	if opts.Step <= 0 {
		return errors.New("sweep step must be positive")
	}

	slog.Info("sweep started", "lights", opts.Lights, "colour", opts.Colour,
		"background", opts.Background, "step", opts.Step, "passes", opts.Passes)

	ticker := time.NewTicker(opts.Step)
	defer ticker.Stop()

	for i, lit := range sweepPositions(len(opts.Lights), opts.Passes) {
		// The first step snaps to the starting pattern; later steps fade
		// over the whole step so the colour moves smoothly.
		fade := opts.Step.Seconds()
		if i == 0 {
			fade = 0
		}

		req := sweepFrame(opts.Lights, lit, opts.Colour, opts.Background, fade)
		if err := sendToLIFX(ctx, http.MethodPut, token, apiBase+"/lights/states", req); err != nil {
			return fmt.Errorf("sweep step %d: %w", i, err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}

	if err := activateAllWarm(ctx, token); err != nil {
		return err
	}
	slog.Info("sweep finished")
	return nil
}

// Sweep returns a handler that starts a sweep across the lamps, e.g.
// POST /lights/sweep?colour=F47A38&step=0.5. It responds 202 straight away
// and runs the sweep in the background, 400 for a bad parameter, or 409 if a
// sweep is already running.
//
// Query parameters, all optional: colour and background (hex, default red
// and blue), step (seconds per lamp, default 0.2) and passes (default 3).
//
// ctx should be the server's lifetime context, not the request's: the request
// context is cancelled as soon as the response is written, which would stop
// the sweep.
func Sweep(ctx context.Context, token string) http.HandlerFunc {
	var running atomic.Bool

	return func(w http.ResponseWriter, r *http.Request) {
		opts := SweepOptions{
			Lights:     sweepLights,
			Colour:     red,
			Background: blue,
			Step:       200 * time.Millisecond,
			Passes:     3,
		}

		q := newQueryParser(r)
		q.colour("colour", &opts.Colour)
		q.colour("background", &opts.Background)
		q.seconds("step", &opts.Step)
		q.int("passes", &opts.Passes)
		if q.err != nil {
			http.Error(w, q.err.Error(), http.StatusBadRequest)
			return
		}

		if !running.CompareAndSwap(false, true) {
			http.Error(w, "a sweep is already running", http.StatusConflict)
			return
		}

		go func() {
			defer running.Store(false)
			if err := RunSweep(ctx, token, opts); err != nil && !errors.Is(err, context.Canceled) {
				slog.Error("running sweep", "err", err)
			}
		}()

		w.WriteHeader(http.StatusAccepted)
	}
}
