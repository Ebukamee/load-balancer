package lb

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Ebukamee/load-balancer/internal/backend"
	"github.com/Ebukamee/load-balancer/internal/balancer"
	"github.com/Ebukamee/load-balancer/internal/proxy"
)

// startBackends starts one real test server per name. Each replies with its
// own name, so the response body shows which backend served the request.
func startBackends(t *testing.T, names ...string) []*backend.Backend {
	t.Helper()
	var bs []*backend.Backend
	for _, name := range names {
		bs = append(bs, startBackend(t, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, name)
		}))
	}
	return bs
}

func startBackend(t *testing.T, h http.HandlerFunc) *backend.Backend {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return newBackend(t, srv.URL, proxy.Options{})
}

// deadBackend returns a backend whose port refuses connections.
func deadBackend(t *testing.T) *backend.Backend {
	t.Helper()
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	return newBackend(t, srv.URL, proxy.Options{})
}

func newBackend(t *testing.T, raw string, opts proxy.Options) *backend.Backend {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return &backend.Backend{URL: u, Proxy: proxy.NewHandler(u, opts)}
}

func serve(h http.Handler, method string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, "/", nil))
	return rec
}

func TestSpreadsRequestsAcrossBackends(t *testing.T) {
	h := NewHandler(balancer.NewRoundRobin(startBackends(t, "b1", "b2", "b3")), 3)

	var got []string
	for range 6 {
		got = append(got, serve(h, http.MethodGet).Body.String())
	}

	want := "b1 b2 b3 b1 b2 b3"
	if strings.Join(got, " ") != want {
		t.Fatalf("backends that answered = %q, want %q", strings.Join(got, " "), want)
	}
}

func TestNoBackendsReturns503(t *testing.T) {
	h := NewHandler(balancer.NewRoundRobin(nil), 3)

	rec := serve(h, http.MethodGet)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if !strings.Contains(rec.Body.String(), "no backends available") {
		t.Errorf("body = %q, want it to contain %q", rec.Body.String(), "no backends available")
	}
}

// The first backend in line is dead. The client must not notice: the LB
// retries on b2, and marks the dead one down so later requests skip it.
func TestRetriesGETOnAnotherBackend(t *testing.T) {
	dead := deadBackend(t)
	bs := append([]*backend.Backend{dead}, startBackends(t, "b2")...)
	h := NewHandler(balancer.NewRoundRobin(bs), 3)

	rec := serve(h, http.MethodGet)

	if rec.Code != http.StatusOK || rec.Body.String() != "b2" {
		t.Fatalf("got %d %q, want 200 %q (retry on the next backend)", rec.Code, rec.Body.String(), "b2")
	}
	if dead.IsAlive() {
		t.Error("the dead backend is still marked alive; mark it down after a failure (passive health check)")
	}
}

// POST is not safe to repeat, so it must NOT be retried, even though b2 is
// healthy. The client gets the 502 and decides for itself.
func TestDoesNotRetryPOST(t *testing.T) {
	var hits atomic.Int32
	live := startBackend(t, func(_ http.ResponseWriter, _ *http.Request) { hits.Add(1) })
	h := NewHandler(balancer.NewRoundRobin([]*backend.Backend{deadBackend(t), live}), 3)

	rec := serve(h, http.MethodPost)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}
	if hits.Load() != 0 {
		t.Errorf("POST was resent to another backend %d time(s); POST must never be retried", hits.Load())
	}
}

func TestAllBackendsDeadReturns502(t *testing.T) {
	bs := []*backend.Backend{deadBackend(t), deadBackend(t)}
	h := NewHandler(balancer.NewRoundRobin(bs), 3)

	rec := serve(h, http.MethodGet)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}
	for i, b := range bs {
		if b.IsAlive() {
			t.Errorf("backend %d still marked alive after failing", i)
		}
	}
}

func TestStopsAfterMaxAttempts(t *testing.T) {
	bs := []*backend.Backend{deadBackend(t), deadBackend(t), deadBackend(t)}
	h := NewHandler(balancer.NewRoundRobin(bs), 2)

	serve(h, http.MethodGet)

	tried := 0
	for _, b := range bs {
		if !b.IsAlive() {
			tried++
		}
	}
	if tried != 2 {
		t.Errorf("%d backends were tried, want exactly 2 (maxAttempts)", tried)
	}
}

func TestSlowBackendReturns504(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(srv.Close)
	defer close(release)

	slow := newBackend(t, srv.URL, proxy.Options{ResponseTimeout: 50 * time.Millisecond})
	h := NewHandler(balancer.NewRoundRobin([]*backend.Backend{slow}), 1)

	rec := serve(h, http.MethodGet)

	if rec.Code != http.StatusGatewayTimeout {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusGatewayTimeout)
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		method string
		want   bool
	}{
		{http.MethodGet, true},
		{http.MethodHead, true},
		{http.MethodOptions, true},
		{http.MethodPost, false},
		{http.MethodPut, false},
		{http.MethodPatch, false},
		{http.MethodDelete, false},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			r := httptest.NewRequest(tt.method, "/", nil)
			if got := isRetryable(r); got != tt.want {
				t.Errorf("isRetryable(%s) = %v, want %v", tt.method, got, tt.want)
			}
		})
	}
}
