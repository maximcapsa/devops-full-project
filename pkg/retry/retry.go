// Package retry provides a small context-aware exponential backoff helper used
// by producers and consumer->DB writes, so a transient Kafka/DB hiccup doesn't
// lose work.
package retry

import (
	"context"
	"time"
)

// Do runs fn up to attempts times. After each failure it waits, starting at
// base and doubling up to max, until fn succeeds, attempts are exhausted, or
// ctx is cancelled. It returns nil on success, ctx.Err() on cancellation, or
// the last error from fn.
func Do(ctx context.Context, attempts int, base, max time.Duration, fn func() error) error {
	if attempts < 1 {
		attempts = 1
	}
	var err error
	delay := base
	for i := 0; i < attempts; i++ {
		if err = fn(); err == nil {
			return nil
		}
		if i == attempts-1 {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
		if delay *= 2; delay > max {
			delay = max
		}
	}
	return err
}
