package lb

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

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
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, name)
		}))
		t.Cleanup(srv.Close)
		u, err := url.Parse(srv.URL)
		if err != nil {
			t.Fatal(err)
		}
		bs = append(bs, &backend.Backend{URL: u, Proxy: proxy.NewHandler(u)})
	}
	return bs
}

func TestSpreadsRequestsAcrossBackends(t *testing.T) {
	h := NewHandler(balancer.NewRoundRobin(startBackends(t, "b1", "b2", "b3")))

	var got []string
	for range 6 {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		got = append(got, rec.Body.String())
	}

	want := "b1 b2 b3 b1 b2 b3"
	if strings.Join(got, " ") != want {
		t.Fatalf("backends that answered = %q, want %q", strings.Join(got, " "), want)
	}
}

func TestNoBackendsReturns503(t *testing.T) {
	h := NewHandler(balancer.NewRoundRobin(nil))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if !strings.Contains(rec.Body.String(), "no backends available") {
		t.Errorf("body = %q, want it to contain %q", rec.Body.String(), "no backends available")
	}
}
