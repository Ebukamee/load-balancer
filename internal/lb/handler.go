// Package lb ties the pieces together: for each request it asks a
// Balancer for a backend and hands the request to that backend's proxy.
package lb

import (
	"log"
	"net/http"

	"github.com/Ebukamee/load-balancer/internal/balancer"
	"github.com/Ebukamee/load-balancer/internal/proxy"
)

// NewHandler returns the load balancer's top-level HTTP handler. If a
// backend cannot be reached, it is marked dead at once (passive health
// check) and safe requests are retried on another backend, up to
// maxAttempts tries in total.
func NewHandler(bal balancer.Balancer, maxAttempts int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var lastErr error

		for attempt := 1; attempt <= maxAttempts; attempt++ {
			b, err := bal.Next()
			if err != nil {
				if lastErr == nil {
					log.Printf("pick backend: %v", err)
					http.Error(w, "no backends available", http.StatusServiceUnavailable)
					return
				}
				break
			}

			req, failure := proxy.WithFailure(r)
			b.Proxy.ServeHTTP(w, req)
			if failure.Err == nil {
				return
			}

			log.Printf("attempt %d: %s failed: %v", attempt, b.URL, failure.Err)
			b.SetAlive(false)
			lastErr = failure.Err

			if !isRetryable(r) {
				break
			}
		}

		proxy.WriteError(w, lastErr)
	})
}

func isRetryable(r *http.Request) bool {

	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}
