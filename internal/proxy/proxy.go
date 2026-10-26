// Package proxy holds the HTTP handler that sits between clients and
// backends. In Step 1 it only answers 503; Step 2 turns it into a real
// reverse proxy.
package proxy

import "net/http"

// NewHandler returns the load balancer's HTTP handler.
func NewHandler() http.Handler {
	// Formatted
	// TODO(step 2): forward requests to a backend with httputil.ReverseProxy.
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "no backends configured yet", http.StatusServiceUnavailable)
	})
}
