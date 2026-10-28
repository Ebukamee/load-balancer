package balancer

import (
	"sync/atomic"

	"github.com/Ebukamee/load-balancer/internal/backend"
)

// RoundRobin hands out backends in a fixed rotation: b1, b2, b3, b1, ...
type RoundRobin struct {
	backends []*backend.Backend
	counter  atomic.Uint64
}

// NewRoundRobin returns a RoundRobin over backends, starting with the first.
func NewRoundRobin(backends []*backend.Backend) *RoundRobin {
	return &RoundRobin{backends: backends}
}

// Next returns the next backend in the rotation, or ErrNoBackends if the
// list is empty. It is safe for concurrent use.
func (rr *RoundRobin) Next() (*backend.Backend, error) {
	if len(rr.backends) == 0 {
		return nil, ErrNoBackends
	}
	n := rr.counter.Add(1)
	i := n - 1
	available := i % uint64(len(rr.backends))
	return rr.backends[available], nil
}
