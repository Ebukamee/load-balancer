// Package lb ties the pieces together: for each request it asks a
// Balancer for a backend and hands the request to that backend's proxy.
package lb

import (
	"log"
	"net/http"

	"github.com/Ebukamee/load-balancer/internal/balancer"
)

// NewHandler returns the load balancer's top-level HTTP handler.
func NewHandler(bal balancer.Balancer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := bal.Next()

		if err != nil {
			log.Printf("pick backend: %v", err)
			http.Error(w, "no backends available", http.StatusServiceUnavailable)
			return
		}
		b.Proxy.ServeHTTP(w, r)
	})
}
