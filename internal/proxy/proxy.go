// Package proxy holds the HTTP handler that sits between clients and
// backends. In Step 2 it forwards every request to a single backend.
package proxy

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

// NewHandler returns a handler that forwards every request to target
// (for example http://localhost:7001) and copies the response back.
//
// Requirements (see proxy_test.go):
//   - the backend's status, headers and body reach the client
//   - method, path, query and body reach the backend unchanged
//   - the backend receives X-Forwarded-For with the client's IP
//   - if the backend is unreachable the client gets 502 with a body
//     containing "backend unavailable"
func NewHandler(target *url.URL) http.Handler {
	return &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.SetXForwarded()
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			log.Printf("Err: %v", err)
			http.Error(w, "backend unavailable", http.StatusBadGateway)
		},
	}
}
