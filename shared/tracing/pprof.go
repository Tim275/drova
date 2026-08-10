package tracing

import (
	"net/http"
	"net/http/pprof"

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

	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Warnw("pprof server error", zap.Error(err))
		}
	}()
}
