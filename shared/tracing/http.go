package tracing

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func WrapHandlerFunc(handler http.HandlerFunc, operation string) http.Handler {
	return otelhttp.NewHandler(handler, operation)
}

func WrapHandler(handler http.Handler, operation string) http.Handler {
	return otelhttp.NewHandler(handler, operation)
}
