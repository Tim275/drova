package middleware

import (
	"expvar"
	"net/http"
	"sync/atomic"
)

var (
	totalRequestsReceived = expvar.NewInt("total_requests_received")
	totalResponsesSent    = expvar.NewInt("total_responses_sent")
	activeRequests        atomic.Int64
)

func init() {
	expvar.Publish("active_requests", expvar.Func(func() any {
		return activeRequests.Load()
	}))
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		totalRequestsReceived.Add(1)
		activeRequests.Add(1)
		defer activeRequests.Add(-1)

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		totalResponsesSent.Add(1)
	})
}
