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

// sweepRed is the moving colour and sweepBlue the background. Red uses a hue
// of 360 rather than 0, because Colour.String drops zero fields.
var (
	sweepRed  = Colour{Hue: 360, Saturation: 1, Brightness: 1}
	sweepBlue = Colour{Hue: 240, Saturation: 1, Brightness: 1}
)

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

// RunSweep moves opts.Colour along opts.Lights and back, with every other
// light set to opts.Background. Each step fades the old light to the
// background as the next light fades in. It blocks until the sweep ends,
// then activates the all-warm scene.
//
// LIFX allows 120 requests a minute, and each step is one request, so keep
// Step at 0.5 seconds or more.
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

	ticker := time.NewTicker(opts.Step)
	defer ticker.Stop()

	for i, lit := range sweepPositions(len(opts.Lights), opts.Passes) {
		// The first step snaps to the starting pattern; later steps fade
		// over the whole step so the colour moves smoothly.
		duration := opts.Step.Seconds()
		if i == 0 {
			duration = 0
		}

		req := statesRequest{States: make([]SelectorState, len(opts.Lights))}
		for j, label := range opts.Lights {
			c := opts.Background
			if j == lit {
				c = opts.Colour
			}
			req.States[j] = SelectorState{
				Selector: MakeSelector(Label, label),
				LightState: LightState{
					Colour:   c,
					Duration: &duration,
					// Don't wait for LIFX to confirm each bulb; keeps steps on time.
					Fast: true,
				},
			}
		}

		if err := sendToLIFX(ctx, http.MethodPut, token, "https://api.lifx.com/v1/lights/states", req); err != nil {
			return fmt.Errorf("sweep step %d: %w", i, err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}

	if err := ActivateScene(ctx, token, allWarmScene, SceneOptions{}); err != nil {
		return fmt.Errorf("activating scene: %w", err)
	}
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
			Colour:     sweepRed,
			Background: sweepBlue,
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
