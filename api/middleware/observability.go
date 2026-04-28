package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
)

var infraPaths = map[string]bool{
	"/livez":   true,
	"/readyz":  true,
	"/metrics": true,
}

func Observability(logger *logrus.Entry) gin.HandlerFunc {
	return func(c *gin.Context) {
		if infraPaths[c.Request.URL.Path] {
			c.Next()
			return
		}

		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		if raw != "" {
			path = path + "?" + raw
		}

		statusCode := c.Writer.Status()
		fields := logrus.Fields{
			"status":        statusCode,
			"latency":       time.Since(start),
			"ip":            c.ClientIP(),
			"method":        c.Request.Method,
			"path":          path,
			"upload_size":   max(c.Request.ContentLength, 0),
			"download_size": max(c.Writer.Size(), 0),
		}

		if spanCtx := trace.SpanFromContext(c.Request.Context()).SpanContext(); spanCtx.IsValid() {
			fields["trace_id"] = spanCtx.TraceID()
			fields["span_id"] = spanCtx.SpanID()
		}

		if msg := c.Errors.ByType(gin.ErrorTypePrivate).String(); msg != "" {
			fields["error"] = msg
		}

		entry := logger.WithFields(fields)
		switch {
		case statusCode >= 500:
			entry.Error("request failed")
		case statusCode >= 400:
			entry.Warn("request error")
		default:
			entry.Info("request processed")
		}
	}
}
