// Package health serves Kubernetes liveness/readiness endpoints. /healthz is a
// liveness probe (200 while the process runs); /readyz runs registered checks
// (DB ping, Kafka ping) and returns 503 if any fail.
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// CheckFunc reports readiness of one dependency.
type CheckFunc func(ctx context.Context) error

// Server holds readiness checks and exposes an HTTP mux.
type Server struct {
	mu    sync.RWMutex
	ready map[string]CheckFunc
	mux   *http.ServeMux
}

// New returns a health server with /healthz and /readyz wired up.
func New() *Server {
	s := &Server{ready: map[string]CheckFunc{}, mux: http.NewServeMux()}
	s.mux.HandleFunc("/healthz", s.handleHealthz)
	s.mux.HandleFunc("/readyz", s.handleReadyz)
	return s
}

// AddReadyCheck registers a named readiness check.
func (s *Server) AddReadyCheck(name string, fn CheckFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ready[name] = fn
}

// Mux exposes the underlying mux so services can mount extra routes (e.g.
// /metrics or the grpc-gateway REST handler).
func (s *Server) Mux() *http.ServeMux { return s.mux }

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	checks := make(map[string]CheckFunc, len(s.ready))
	for k, v := range s.ready {
		checks[k] = v
	}
	s.mu.RUnlock()

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	ok := true
	results := make(map[string]string, len(checks))
	for name, fn := range checks {
		if err := fn(ctx); err != nil {
			ok = false
			results[name] = err.Error()
		} else {
			results[name] = "ok"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if ok {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": ok, "checks": results})
}
