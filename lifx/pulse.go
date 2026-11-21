package lifx

import (
	"context"
	"fmt"
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
	if err := SetPulse(ctx, token, selector, opts); err != nil {
		return fmt.Errorf("pulsing: %w", err)
	}

	// The pulse API returns as soon as the effect starts. Resetting straight
	// away would cut the pulse off, so wait for it to end.
	wait := time.Duration(opts.Period * opts.Cycles * float64(time.Second))
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(wait):
	}

	return ResetLights(ctx, token, selector)
}
