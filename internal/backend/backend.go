// Package backend describes one upstream server the load balancer can
// send traffic to.
package backend

import (
	"net/http"
	"net/url"
)

// Backend is one upstream server. Later steps add health state (Step 5)
// and in-flight request counts (Step 7) here.
type Backend struct {
	URL *url.URL

	// Proxy forwards a request to URL. It is built once per backend
	// (with proxy.NewHandler) and reused for every request.
	Proxy http.Handler
}
