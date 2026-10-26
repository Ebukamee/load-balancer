// Command lb is the load balancer entry point. It stays deliberately thin:
// parse flags, build the handler from internal packages, start the server.
package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/ebuka/loadbalancer/internal/proxy"
)

func main() {
	addr := flag.String("addr", ":8080", "address the load balancer listens on")
	flag.Parse()

	srv := &http.Server{
		Addr:              *addr,
		Handler:           proxy.NewHandler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("load balancer listening on %s", *addr)
	log.Fatal(srv.ListenAndServe())
}
