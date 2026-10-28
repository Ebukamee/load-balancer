// Package backend describes one upstream server the load balancer can
// send traffic to.
package backend

import (
	"net/http"
	"net/url"
	"sync/atomic"
)

// Backend is one upstream server. Step 7 adds in-flight request counts here.
type Backend struct {
	URL *url.URL

	// Proxy forwards a request to URL. It is built once per backend
	// (with proxy.NewHandler) and reused for every request.
	Proxy http.Handler

	// down is written by the health checker goroutine and read by every
	// request goroutine at the same time, so it must be atomic (see Step 3).
	// It is stored as "down" rather than "alive" so that the zero value
	// means alive: a new backend gets traffic until a check says otherwise.
	down atomic.Bool
}

// IsAlive reports whether the last health check passed.
func (b *Backend) IsAlive() bool { return !b.down.Load() }

// SetAlive records the result of a health check.
func (b *Backend) SetAlive(alive bool) { b.down.Store(!alive) }
