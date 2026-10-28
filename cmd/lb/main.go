// Command lb is the load balancer entry point. It stays deliberately thin:
// parse flags, build the handler from internal packages, start the server.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Ebukamee/load-balancer/internal/backend"
	"github.com/Ebukamee/load-balancer/internal/balancer"
	"github.com/Ebukamee/load-balancer/internal/lb"
	"github.com/Ebukamee/load-balancer/internal/proxy"
)

func main() {
	addr := flag.String("addr", ":8080", "address the load balancer listens on")
	backendList := flag.String("backends",
		"http://localhost:7001,http://localhost:7002,http://localhost:7003",
		"comma-separated backend URLs")
	flag.Parse()

	backends, err := parseBackends(*backendList)
	if err != nil {
		log.Fatal(err)
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           lb.NewHandler(balancer.NewRoundRobin(backends)),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("load balancer listening on %s, %d backends: %s", *addr, len(backends), *backendList)
	log.Fatal(srv.ListenAndServe())
}

// parseBackends turns "http://a:1,http://b:2" into backends, each with its
// own reverse proxy.
func parseBackends(list string) ([]*backend.Backend, error) {
	var backends []*backend.Backend
	for _, raw := range strings.Split(list, ",") {
		raw = strings.TrimSpace(raw)
		u, err := url.Parse(raw)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return nil, fmt.Errorf("invalid backend %q: want something like http://localhost:7001", raw)
		}
		backends = append(backends, &backend.Backend{URL: u, Proxy: proxy.NewHandler(u)})
	}
	return backends, nil
}
