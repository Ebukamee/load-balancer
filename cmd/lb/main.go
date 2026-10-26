// Command lb is the load balancer entry point. It stays deliberately thin:
// parse flags, build the handler from internal packages, start the server.
package main

import (
	"flag"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/Ebukamee/load-balancer/internal/proxy"
)

func main() {
	addr := flag.String("addr", ":8080", "address the load balancer listens on")
	backend := flag.String("backend", "http://localhost:7001", "URL of the backend to forward to")
	flag.Parse()

	target, err := url.Parse(*backend)
	if err != nil || target.Scheme == "" || target.Host == "" {
		log.Fatalf("invalid -backend %q: want something like http://localhost:7001", *backend)
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           proxy.NewHandler(target),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("load balancer listening on %s, forwarding to %s", *addr, target)
	log.Fatal(srv.ListenAndServe())
}
