// Package config reads 12-factor configuration from environment variables.
// No config files, no secrets on disk — services compose their own typed
// config struct from these helpers.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// String returns the env var or def if unset.
func String(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

// MustString returns the env var or an error if it is unset/empty.
func MustString(key string) (string, error) {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v, nil
	}
	return "", fmt.Errorf("required env var %s is not set", key)
}

// Int returns the env var parsed as int, or def on unset/parse error.
func Int(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// Duration returns the env var parsed as a Go duration, or def.
func Duration(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

// Strings splits a comma-separated env var, trimming whitespace; def on unset.
func Strings(key string, def []string) []string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		parts := strings.Split(v, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		return out
	}
	return def
}
