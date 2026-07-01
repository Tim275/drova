// Package debugserver runs an internal "ops" HTTP server that exposes the
// Prometheus /metrics scrape endpoint and, optionally, Go's pprof profiler.
//
// It is deliberately a SEPARATE port from the public API so that metrics and
// profiles stay internal (scraped in-cluster, never routed through the public
// ingress).
package debugserver

import (
	"context"
	"net/http"
	"net/http/pprof" //nolint:gosec // G108: pprof is bound to the internal ops port, never the public mux
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Start launches the ops server on addr (e.g. ":9464") in a background goroutine
// and returns a stop function. addr == "" disables it (no-op stop).
//
//   - /metrics            Prometheus scrape endpoint (always on)
//   - /debug/pprof/*      profiling endpoints (only when withPprof is true)
//
// Profile with:
//
//	go tool pprof http://localhost:9464/debug/pprof/heap
//	go tool pprof http://localhost:9464/debug/pprof/profile?seconds=30
func Start(addr string, withPprof bool) func() {
	if addr == "" {
		return func() {}
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	if withPprof {
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		_ = srv.ListenAndServe()
	}()

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}
}
