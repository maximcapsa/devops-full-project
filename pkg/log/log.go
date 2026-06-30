// Package log builds the standard structured JSON logger used by every service.
package log

import (
	"log/slog"
	"os"
	"strings"
)

// New returns a slog.Logger writing JSON to stdout at the given level
// ("debug"|"info"|"warn"|"error"), and installs it as the slog default.
func New(level string) *slog.Logger {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: ParseLevel(level)})
	l := slog.New(h)
	slog.SetDefault(l)
	return l
}

// ParseLevel maps a level string to slog.Level, defaulting to Info.
func ParseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
