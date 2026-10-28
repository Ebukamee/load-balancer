// Package lb ties the pieces together: for each request it asks a
// Balancer for a backend and hands the request to that backend's proxy.
package lb

import (
	"net/http"
	"log"

	"github.com/Ebukamee/load-balancer/internal/balancer"
)

// NewHandler returns the load balancer's top-level HTTP handler.
func NewHandler(bal balancer.Balancer) http.Handler {
	// TODO(step 3): everything below happens INSIDE the func(w, r) { ... }
	// at the bottom, because it must run once per request, not once at start.
	//
	// STEP A: delete the `_ = bal` line, and rename the `_` parameter to `r`
	// (you need the request now to pass it on).
	//
	// STEP B: replace the http.Error placeholder with a pick:
	//
	//	b, err := bal.Next()
	//
	// STEP C: if err != nil, the balancer had nothing to give:
	//   - log it:   log.Printf("pick backend: %v", err)   (import "log")
	//   - reply:    http.Error(w, "no backends available", http.StatusServiceUnavailable)
	//   - then `return`, or the code below runs with b == nil and panics.
	//
	// STEP D: otherwise hand the request to the chosen backend's proxy (the
	// ReverseProxy you wrote in Step 2, stored in b.Proxy):
	//
	//	b.Proxy.ServeHTTP(w, r)
	//
	// Then run: go test ./internal/lb -v
	_ = bal
	return http.HandlerFunc(func(w http.ResponseWriter, r*http.Request) {
		b,err := bal.Next()

		if err != nil {
			log.Printf("pick backend: %v", err)
			http.Error(w,"no backends available",http.StatusServiceUnavailable)
			return
		}
		b.Proxy.ServeHTTP(w, r)
	})
}
