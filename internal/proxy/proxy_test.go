package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// newBackend starts a real HTTP server on a random local port to act as the
// upstream, and stops it when the test ends. It returns the server's URL.
func newBackend(t *testing.T, h http.HandlerFunc) *url.URL {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// send pushes one request through the load balancer handler and returns
// the recorded response.
func send(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestForwardsResponseFromBackend(t *testing.T) {
	backend := newBackend(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Backend", "b1")
		w.WriteHeader(http.StatusTeapot)
		_, _ = io.WriteString(w, "hello from b1")
	})

	rec := send(NewHandler(backend, Options{}), httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusTeapot {
		t.Errorf("status = %d, want %d (the backend's status must reach the client)", rec.Code, http.StatusTeapot)
	}
	if got := rec.Body.String(); got != "hello from b1" {
		t.Errorf("body = %q, want %q", got, "hello from b1")
	}
	if got := rec.Header().Get("X-Backend"); got != "b1" {
		t.Errorf("X-Backend header = %q, want %q (backend headers must reach the client)", got, "b1")
	}
}

func TestKeepsPathQueryAndMethod(t *testing.T) {
	backend := newBackend(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_, _ = fmt.Fprintf(w, "%s %s %s", r.Method, r.URL.RequestURI(), body)
	})

	req := httptest.NewRequest(http.MethodPost, "/orders/42?expand=items", strings.NewReader("payload"))
	rec := send(NewHandler(backend, Options{}), req)

	want := "POST /orders/42?expand=items payload"
	if got := rec.Body.String(); got != want {
		t.Errorf("backend saw %q, want %q", got, want)
	}
}

func TestSetsXForwardedFor(t *testing.T) {
	backend := newBackend(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.Header.Get("X-Forwarded-For"))
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.7:51234" // the client's IP:port as the LB sees it

	rec := send(NewHandler(backend, Options{}), req)

	if got := rec.Body.String(); got != "203.0.113.7" {
		t.Errorf("backend saw X-Forwarded-For = %q, want %q (backends need the real client IP)", got, "203.0.113.7")
	}
}

func TestBackendDownReturns502(t *testing.T) {
	// Start a backend and immediately stop it: its port now refuses connections.
	srv := httptest.NewServer(http.NotFoundHandler())
	dead, _ := url.Parse(srv.URL)
	srv.Close()

	rec := send(NewHandler(dead, Options{}), httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}
	if !strings.Contains(rec.Body.String(), "backend unavailable") {
		t.Errorf("body = %q, want it to contain %q (use a custom ErrorHandler)", rec.Body.String(), "backend unavailable")
	}
}

// timeoutErr is a fake network error that says it was a timeout, like the
// errors the Transport returns when a dial or response timeout fires.
type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

func TestErrorStatus(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"context deadline", context.DeadlineExceeded, http.StatusGatewayTimeout},
		{"wrapped context deadline", fmt.Errorf("round trip: %w", context.DeadlineExceeded), http.StatusGatewayTimeout},
		{"network timeout", &net.OpError{Op: "dial", Net: "tcp", Err: timeoutErr{}}, http.StatusGatewayTimeout},
		{"connection refused", &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}, http.StatusBadGateway},
		{"anything else", errors.New("boom"), http.StatusBadGateway},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ErrorStatus(tt.err); got != tt.want {
				t.Errorf("ErrorStatus(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

func TestSlowBackendReturns504(t *testing.T) {
	release := make(chan struct{})
	backend := newBackend(t, func(_ http.ResponseWriter, r *http.Request) {
		select { // hang until the test ends or the LB gives up
		case <-release:
		case <-r.Context().Done():
		}
	})
	defer close(release)

	h := NewHandler(backend, Options{ResponseTimeout: 50 * time.Millisecond})
	rec := send(h, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusGatewayTimeout {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusGatewayTimeout)
	}
	if !strings.Contains(rec.Body.String(), "backend timed out") {
		t.Errorf("body = %q, want it to contain %q", rec.Body.String(), "backend timed out")
	}
}

// With a Failure recorder attached, the proxy must not write anything on
// failure: the caller may still retry on another backend.
func TestFailureIsRecordedNotWritten(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	dead, _ := url.Parse(srv.URL)
	srv.Close()

	req, failure := WithFailure(httptest.NewRequest(http.MethodGet, "/", nil))
	rec := send(NewHandler(dead, Options{}), req)

	if failure.Err == nil {
		t.Fatal("failure.Err = nil, want the connection error")
	}
	if rec.Body.Len() != 0 {
		t.Errorf("proxy wrote %q to the client; with a recorder it must write nothing", rec.Body.String())
	}
}
