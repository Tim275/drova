package logger

import (
	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New builds a zap logger that writes to stdout and, if an OTEL log provider
// has been set globally (via tracing.InitTracer), also forwards to it.
// Call tracing.InitTracer before New so the OTEL bridge is active from the start.
func New(env, serviceName string) *zap.SugaredLogger {
	var cfg zap.Config
	if env == "production" {
		cfg = zap.NewProductionConfig()
	} else {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	base, err := cfg.Build()
	if err != nil {
		panic("failed to init logger: " + err.Error())
	}

	otelCore := otelzap.NewCore(serviceName)
	teed := zap.New(zapcore.NewTee(base.Core(), otelCore), zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	return teed.Sugar()
}
