package lifx

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// SetPulse runs a pulse effect on the lights matching selector, switching
// sharply between FromColour (or the current colour) and Colour. Peak is
// ignored. It returns as soon as the effect starts.
func SetPulse(ctx context.Context, token, selector string, opts WaveformOptions) error {
	return setWaveform(ctx, token, selector, "pulse", opts)
}

// CelebrateGoal pulses the lights matching selector between from and to,
// waits for the pulse to finish, then resets the lights. It blocks for the
// length of the pulse.
func CelebrateGoal(ctx context.Context, token, selector string, from, to Colour) error {
	opts := WaveformOptions{
		Colour:     to,
		FromColour: &from,
		Period:     0.5,
		Cycles:     15,
	}
	slog.Info("celebrating goal", "selector", selector, "from", from, "to", to)
	if err := SetPulse(ctx, token, selector, opts); err != nil {
		return fmt.Errorf("pulsing: %w", err)
	}

	// The pulse API returns as soon as the effect starts. Resetting straight
	// away would cut the pulse off, so wait for it to end.
	wait := time.Duration(opts.Period * opts.Cycles * float64(time.Second))
	select {
	case <-ctx.Done():
		return fmt.Errorf("cancelled mid-celebration, lights not reset: %w", ctx.Err())
	case <-time.After(wait):
	}

	if err := ResetLights(ctx, token, selector); err != nil {
		return fmt.Errorf("resetting after celebration: %w", err)
	}
	slog.Info("goal celebration finished", "selector", selector)
	return nil
}
