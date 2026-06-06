// Package wait replaces COBSWAIT.cbl / WAITSTEP.jcl (RAU-44).
// It sleeps for the specified number of centiseconds (1/100th of a second).
package wait

import (
	"context"
	"fmt"
	"time"
)

// Config holds the parameters for the wait subcommand.
type Config struct {
	// Centiseconds is the sleep duration in 1/100-second units (COBSWAIT unit).
	Centiseconds int
}

// Summary reports how long the wait lasted.
type Summary struct {
	Waited time.Duration
}

// Run sleeps for cfg.Centiseconds centiseconds, respecting ctx cancellation.
func Run(ctx context.Context, cfg Config) (Summary, error) {
	if cfg.Centiseconds < 0 {
		return Summary{}, fmt.Errorf("wait: centiseconds must be >= 0, got %d", cfg.Centiseconds)
	}
	d := time.Duration(cfg.Centiseconds) * 10 * time.Millisecond
	select {
	case <-ctx.Done():
		return Summary{}, ctx.Err()
	case <-time.After(d):
	}
	return Summary{Waited: d}, nil
}
