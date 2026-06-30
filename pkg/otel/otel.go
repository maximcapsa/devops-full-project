// Package otel is a placeholder for OpenTelemetry wiring, fleshed out in
// Phase 10. It exposes the final API now (Init returning a shutdown func) so
// service main() code doesn't change later.
package otel

import "context"

// Init currently installs nothing and returns a no-op shutdown. Phase 10
// replaces the body with real tracer/metrics providers.
func Init(_ context.Context, _ string) (shutdown func(context.Context) error, err error) {
	return func(context.Context) error { return nil }, nil
}
