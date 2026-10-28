// Package health runs active health checks: it asks every backend
// "are you OK?" on a timer and marks each one alive or dead.
package health

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/Ebukamee/load-balancer/internal/backend"
)

// Checker periodically probes each backend's health endpoint.
type Checker struct {
	backends []*backend.Backend
	client   *http.Client
	path     string        // endpoint to probe, e.g. "/health"
	interval time.Duration // time between rounds of checks
	timeout  time.Duration // how long one check may take before it counts as failed
}

// New returns a Checker that probes GET <backend URL>/health every interval,
// giving each probe at most timeout to answer.
func New(backends []*backend.Backend, interval, timeout time.Duration) *Checker {
	return &Checker{
		backends: backends,
		client:   &http.Client{},
		path:     "/health",
		interval: interval,
		timeout:  timeout,
	}
}

// Run checks all backends once straight away, then again every interval,
// until ctx is cancelled. It blocks, so start it with: go checker.Run(ctx)
func (c *Checker) Run(ctx context.Context) {
	c.CheckAll(ctx)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.CheckAll(ctx)
		}

	}

	// Then run: go test ./internal/health -v -run Run
}

// CheckAll probes every backend at the same time and records the results.
// It returns when all probes have finished.
func (c *Checker) CheckAll(ctx context.Context) {
	var wg sync.WaitGroup
	for _, b := range c.backends {
		// One goroutine per backend, so one slow backend cannot delay the
		// others. wg.Go starts the goroutine and tracks it; wg.Wait blocks
		// until every one has returned.
		wg.Go(func() {
			alive := c.check(ctx, b)
			if alive != b.IsAlive() {
				log.Printf("health: %s is now %s", b.URL, upDown(alive))
			}
			b.SetAlive(alive)
		})
	}
	wg.Wait()
}

// check sends one GET to the backend's health endpoint. It returns true only
// if the backend answers 2xx within c.timeout.
func (c *Checker) check(ctx context.Context, b *backend.Backend) bool {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	url := b.URL.JoinPath(c.path).String()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)

	if err != nil {
		return false
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func upDown(alive bool) string {
	if alive {
		return "UP"
	}
	return "DOWN"
}
