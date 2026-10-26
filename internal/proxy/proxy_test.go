package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerWithoutBackendsReturns503(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	NewHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}
