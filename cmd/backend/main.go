// Command backend is a tiny demo HTTP server used as an upstream for the
// load balancer. Run several copies on different ports to have something
// to balance across:
//
//	go run ./cmd/backend -addr :7001 -name b1
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"
)

func main() {
	addr := flag.String("addr", ":7001", "address to listen on")
	name := flag.String("name", "b1", "name reported in responses")
	delay := flag.Duration("delay", 0, "artificial delay per request (e.g. 200ms) to simulate a slow server")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(*delay)
		fmt.Fprintf(w, "hello from %s (path=%s)\n", *name, r.URL.Path)
	})

	srv := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("backend %s listening on %s", *name, *addr)
	log.Fatal(srv.ListenAndServe())
}
