package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthz(t *testing.T) {
	s := New()
	rr := httptest.NewRecorder()
	s.Mux().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != http.StatusOK {
		t.Errorf("/healthz = %d, want 200", rr.Code)
	}
}

func TestReadyzPassAndFail(t *testing.T) {
	s := New()
	s.AddReadyCheck("ok", func(context.Context) error { return nil })

	rr := httptest.NewRecorder()
	s.Mux().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rr.Code != http.StatusOK {
		t.Errorf("/readyz all-ok = %d, want 200", rr.Code)
	}

	s.AddReadyCheck("db", func(context.Context) error { return errors.New("down") })
	rr = httptest.NewRecorder()
	s.Mux().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("/readyz with failure = %d, want 503", rr.Code)
	}
}
