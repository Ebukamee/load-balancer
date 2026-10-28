package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Ebukamee/load-balancer/internal/backend"
)

// newBackend starts a test server running h and returns a Backend for it.
func newBackend(t *testing.T, h http.HandlerFunc) *backend.Backend {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return &backend.Backend{URL: u}
}

func respond(status int) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) }
}

func TestHealthyBackendIsMarkedAlive(t *testing.T) {
	b := newBackend(t, respond(http.StatusOK))
	b.SetAlive(false) // start as down so the check has to flip it

	New([]*backend.Backend{b}, time.Hour, time.Second).CheckAll(context.Background())

	if !b.IsAlive() {
		t.Fatal("backend answering 200 should be alive")
	}
}

func TestErrorStatusIsMarkedDead(t *testing.T) {
	b := newBackend(t, respond(http.StatusInternalServerError))

	New([]*backend.Backend{b}, time.Hour, time.Second).CheckAll(context.Background())

	if b.IsAlive() {
		t.Fatal("backend answering 500 should be dead")
	}
}

func TestUnreachableBackendIsMarkedDead(t *testing.T) {
	srv := httptest.NewServer(respond(http.StatusOK))
	u, _ := url.Parse(srv.URL)
	srv.Close() // nothing listens on this port any more
	b := &backend.Backend{URL: u}

	New([]*backend.Backend{b}, time.Hour, time.Second).CheckAll(context.Background())

	if b.IsAlive() {
		t.Fatal("backend refusing connections should be dead")
	}
}

func TestProbesTheHealthPath(t *testing.T) {
	var gotPath atomic.Value
	b := newBackend(t, func(_ http.ResponseWriter, r *http.Request) {
		gotPath.Store(r.URL.Path)
	})

	New([]*backend.Backend{b}, time.Hour, time.Second).CheckAll(context.Background())

	if got, _ := gotPath.Load().(string); got != "/health" {
		t.Fatalf("checker requested %q, want %q", got, "/health")
	}
}

// A backend that hangs must not hang the checker: the probe gives up after
// the timeout and the backend counts as dead.
func TestSlowBackendTimesOut(t *testing.T) {
	release := make(chan struct{})
	b := newBackend(t, func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})
	defer close(release)

	start := time.Now()
	New([]*backend.Backend{b}, time.Hour, 50*time.Millisecond).CheckAll(context.Background())
	took := time.Since(start)

	if b.IsAlive() {
		t.Error("backend that never answers should be dead")
	}
	if took > time.Second {
		t.Errorf("CheckAll took %v; the 50ms timeout was not applied", took)
	}
}

// Run must check straight away, keep checking every interval, and return
// promptly once its context is cancelled.
func TestRunChecksRepeatedlyAndStopsOnCancel(t *testing.T) {
	var hits atomic.Int32
	b := newBackend(t, func(_ http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		New([]*backend.Backend{b}, 20*time.Millisecond, time.Second).Run(ctx)
		close(done)
	}()

	time.Sleep(150 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after the context was cancelled")
	}
	if n := hits.Load(); n < 3 {
		t.Fatalf("backend was checked %d times in 150ms with a 20ms interval, want at least 3", n)
	}
}

func TestRunChecksImmediately(t *testing.T) {
	b := newBackend(t, respond(http.StatusInternalServerError))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go New([]*backend.Backend{b}, time.Hour, time.Second).Run(ctx)

	deadline := time.Now().Add(time.Second)
	for b.IsAlive() {
		if time.Now().After(deadline) {
			t.Fatal("Run did not check at startup (with a 1h interval it must not wait for the first tick)")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
