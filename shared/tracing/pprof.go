package tracing

import (
	"net/http"
	_ "net/http/pprof"

	"go.uber.org/zap"
)

// StartPprofServer exposes net/http/pprof on its own listener. Never route
// this port through a K8s Service/HTTPRoute — it's for Alloy to scrape
// directly via Pod IP (profiles.grafana.com annotation), not public traffic.
func StartPprofServer(log *zap.SugaredLogger, addr string) {
	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Warnw("pprof server error", zap.Error(err))
		}
	}()
}
