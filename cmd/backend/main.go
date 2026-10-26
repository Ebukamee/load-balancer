// Command backend is a tiny demo HTTP server used as an upstream for the
// load balancer. Run several copies on different ports to have something
// to balance across:
//
//	go run ./cmd/backend -addr :7001 -name b1
package main

import (
	"flag"
	"fmt"
	"html"
	"log"
	"net/http"
	"time"
)

func main() {
	addr := flag.String("addr", ":7001", "address to listen on")
	name := flag.String("name", "b1", "name reported in responses")
	delay := flag.Duration("delay", 0, "artificial delay per request (e.g. 200ms) to simulate a slow server")
	flag.Parse()

	// Write errors are deliberately ignored below: they only happen when the
	// client has already gone away, and there is nobody left to tell.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Backend", *name)
		time.Sleep(*delay)
		// The path is client input: escape it so it can never be echoed back
		// as runnable HTML (reflected XSS).
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprintf(w, "hello from %s (path=%s)\n", *name, html.EscapeString(r.URL.Path))
	})

	srv := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("backend %s listening on %s", *name, *addr)
	log.Fatal(srv.ListenAndServe())
}
