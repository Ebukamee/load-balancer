// Package balancer contains the algorithms that decide which backend
// serves the next request.
package balancer

import (
	"errors"

	"github.com/Ebukamee/load-balancer/internal/backend"
)

// ErrNoBackends is returned by Next when there is nothing to pick from.
var ErrNoBackends = errors.New("no backends available")

// Balancer picks the backend for the next request. Implementations must be
// safe to call from many goroutines at once: net/http serves every
// connection on its own goroutine.
type Balancer interface {
	Next() (*backend.Backend, error)
}
