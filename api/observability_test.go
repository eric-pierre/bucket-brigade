package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestObservabilityEndpoints(t *testing.T) {
	restApi, _, _ := setupTest(t)
	router := restApi.router

	// Seed a non-infra request so all metric series are populated before checking /metrics.
	// Unobserved HistogramVec series are omitted from the prometheus text format entirely.
	router.ServeHTTP(httptest.NewRecorder(), mustGET("/objects/testbucket/seed"))

	req, _ := http.NewRequest("GET", "/livez", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	req, _ = http.NewRequest("GET", "/readyz", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	req, _ = http.NewRequest("GET", "/metrics", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	metrics := rr.Body.String()
	assert.Contains(t, metrics, "http_requests_total")
	assert.Contains(t, metrics, "http_request_duration_seconds")
	assert.Contains(t, metrics, "http_request_size_bytes")
	assert.Contains(t, metrics, "http_response_size_bytes")
}

func mustGET(path string) *http.Request {
	req, _ := http.NewRequest("GET", path, nil)
	return req
}

func TestPrometheusMetricsRecording(t *testing.T) {
	restApi, _, _ := setupTest(t)
	router := restApi.router

	// Trigger a request on a non-infra path to record metrics
	req, _ := http.NewRequest("GET", "/objects/testbucket/testobject", nil)
	router.ServeHTTP(httptest.NewRecorder(), req)

	req, _ = http.NewRequest("GET", "/metrics", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	metrics := rr.Body.String()
	assert.Contains(t, metrics, `endpoint="/objects/:bucket/:objectId"`)
	assert.Contains(t, metrics, `method="GET"`)
}
