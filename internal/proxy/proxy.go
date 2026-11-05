// Package proxy holds the HTTP handler that forwards a request to one
// backend and copies the response back.
package proxy

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

// Options controls how long the proxy waits on a backend. Zero values mean
// "use the default".
type Options struct {
	ConnectTimeout  time.Duration // max time to open the TCP connection
	ResponseTimeout time.Duration // max time from sending the request to the response headers arriving
}

// Defaults for Options fields left at zero.
const (
	DefaultConnectTimeout  = 2 * time.Second
	DefaultResponseTimeout = 10 * time.Second
)

// NewHandler returns a handler that forwards every request to target
// (for example http://localhost:7001) and copies the response back.
//
// If the backend cannot be reached or is too slow, it replies 502 or 504 —
// unless the request carries a Failure recorder (see WithFailure), in which
// case it writes nothing and records the error so the caller can retry.
func NewHandler(target *url.URL, opts Options) http.Handler {
	if opts.ConnectTimeout == 0 {
		opts.ConnectTimeout = DefaultConnectTimeout
	}
	if opts.ResponseTimeout == 0 {
		opts.ResponseTimeout = DefaultResponseTimeout
	}

	return &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.SetXForwarded()
		},
		// Our own Transport instead of http.DefaultTransport, so we control
		// every timeout. DefaultTransport never gives up waiting for a reply.
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   opts.ConnectTimeout,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ResponseHeaderTimeout: opts.ResponseTimeout,
			IdleConnTimeout:       90 * time.Second,
			// The default keeps only 2 idle connections per backend, so under
			// load most requests would open a brand-new TCP connection.
			MaxIdleConnsPerHost: 100,
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			if f, ok := r.Context().Value(failureKey{}).(*Failure); ok {
				f.Err = err // the caller (lb) decides: retry or reply
				return
			}
			// %q escapes newlines, so request data inside err cannot fake extra log lines.
			log.Printf("proxy to %s: %q", target, err.Error()) //nolint:gosec // %q escapes the error, so it stays on one line
			WriteError(w, err)
		},
	}
}

// Failure records why forwarding failed. Err stays nil on success.
type Failure struct {
	Err error
}

type failureKey struct{}

// WithFailure returns a copy of r carrying a fresh Failure recorder. Serve
// the copy; afterwards, f.Err is nil if the backend answered, or the error
// if it could not be reached — and in that case NOTHING was written to the
// client yet, so it is safe to try another backend.
func WithFailure(r *http.Request) (*http.Request, *Failure) {
	f := &Failure{}
	return r.WithContext(context.WithValue(r.Context(), failureKey{}, f)), f
}

// WriteError replies to the client with the status that matches err.
func WriteError(w http.ResponseWriter, err error) {
	status := ErrorStatus(err)
	msg := "backend unavailable"
	if status == http.StatusGatewayTimeout {
		msg = "backend timed out"
	}
	http.Error(w, msg, status)
}

// ErrorStatus picks the HTTP status for a forwarding error:
// 504 Gateway Timeout if the backend was too slow, otherwise 502 Bad Gateway.
func ErrorStatus(err error) int {

	if errors.Is(err, context.DeadlineExceeded) {
		return http.StatusGatewayTimeout
	}

	var ne net.Error

	if errors.As(err, &ne) && ne.Timeout() {
		return http.StatusGatewayTimeout
	}

	return http.StatusBadGateway
}
