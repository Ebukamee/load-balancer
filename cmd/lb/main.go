// Command lb is the load balancer entry point. It stays deliberately thin:
// load the config, build the pieces from internal packages, start the server.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/Ebukamee/load-balancer/internal/backend"
	"github.com/Ebukamee/load-balancer/internal/balancer"
	"github.com/Ebukamee/load-balancer/internal/config"
	"github.com/Ebukamee/load-balancer/internal/health"
	"github.com/Ebukamee/load-balancer/internal/lb"
	"github.com/Ebukamee/load-balancer/internal/proxy"
)

func main() {
	configPath := flag.String("config", "configs/lb.json", "path to the JSON config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	backends, err := buildBackends(cfg.Backends)
	if err != nil {
		log.Fatal(err)
	}

	// Health checks run in the background for as long as the program runs.
	// Step 9 cancels this context on Ctrl+C for a clean shutdown.
	go health.New(backends, cfg.Health.Interval.Duration, cfg.Health.Timeout.Duration).Run(context.Background())

	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           lb.NewHandler(balancer.NewRoundRobin(backends)),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("load balancer listening on %s with %d backends (config %s)", cfg.Listen, len(backends), *configPath)
	log.Fatal(srv.ListenAndServe())
}

// buildBackends turns validated backend URLs into backends, each with its
// own reverse proxy.
func buildBackends(urls []string) ([]*backend.Backend, error) {
	var backends []*backend.Backend
	for _, raw := range urls {
		u, err := url.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("backend %q: %w", raw, err)
		}
		backends = append(backends, &backend.Backend{URL: u, Proxy: proxy.NewHandler(u)})
	}
	return backends, nil
}
