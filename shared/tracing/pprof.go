package tracing

import (
	"net/http"
	"net/http/pprof"
	"time"

	"go.uber.org/zap"
)

// StartPprofServer exposes net/http/pprof on its own listener, on an explicit
// mux (not http.DefaultServeMux). Never route this port through a K8s
// Service/HTTPRoute — it's for Alloy to scrape directly via Pod IP
// (profiles.grafana.com annotation), not public traffic.
func StartPprofServer(log *zap.SugaredLogger, addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	// WriteTimeout has headroom for CPU-profile captures (client-controlled
	// via ?seconds=N, default 30s) — too tight here cuts profiling off.
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Warnw("pprof server error", zap.Error(err))
		}
	}()
}
