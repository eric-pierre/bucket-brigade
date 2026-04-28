package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "endpoint", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	httpRequestSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_size_bytes",
			Help:    "Size of HTTP requests.",
			Buckets: []float64{100, 1000, 10000, 100000, 1000000, 10000000},
		},
		[]string{"method", "endpoint"},
	)

	httpResponseSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_response_size_bytes",
			Help:    "Size of HTTP responses.",
			Buckets: []float64{100, 1000, 10000, 100000, 1000000, 10000000},
		},
		[]string{"method", "endpoint"},
	)
)

func Prometheus() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.FullPath()

		if infraPaths[path] {
			c.Next()
			return
		}

		start := time.Now()
		method := c.Request.Method

		httpRequestSize.WithLabelValues(method, path).Observe(float64(max(c.Request.ContentLength, 0)))

		c.Next()

		httpRequestDuration.WithLabelValues(method, path).Observe(time.Since(start).Seconds())
		httpResponseSize.WithLabelValues(method, path).Observe(float64(max(c.Writer.Size(), 0)))
		httpRequestsTotal.WithLabelValues(method, path, strconv.Itoa(c.Writer.Status())).Inc()
	}
}
