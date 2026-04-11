// Command ktranslate-topology is a standalone viewer that turns
// KSnmpTopology records emitted by the main ktranslate binary into a
// live, auto-refreshing web page.
//
// Usage:
//
//	ktranslate-topology -listen :8082 -ttl 2h
//
// And point ktranslate's HTTP sink at the ingest endpoint:
//
//	ktranslate -format json -sinks http -http_url http://localhost:8082/ingest ...
//
// Open http://localhost:8082/ to see the graph.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	var (
		listen = flag.String("listen", ":8082", "address to listen on for ingest + UI")
		ttl    = flag.Duration("ttl", 2*time.Hour, "drop links not seen within this window")
	)
	flag.Parse()

	logger := log.New(os.Stderr, "ktranslate-topology ", log.LstdFlags|log.Lmicroseconds)

	g := NewGraph(*ttl)
	srv := &Server{Graph: g, Log: logger}

	httpSrv := &http.Server{
		Addr:              *listen,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	logger.Printf("listening on %s (link ttl: %s)", *listen, ttl)

	// Run the HTTP server in a goroutine so we can react to signals.
	errCh := make(chan error, 1)
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.Printf("received %s, shutting down", sig)
	case err := <-errCh:
		if err != nil {
			logger.Fatalf("http server failed: %v", err)
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(ctx); err != nil {
		logger.Printf("shutdown: %v", err)
	}
}
