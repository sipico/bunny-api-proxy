// Package metrics provides Prometheus metrics collection for the proxy.
package metrics

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Global metrics - used by the application
	requestsTotal      *prometheus.CounterVec
	requestDuration    *prometheus.HistogramVec
	authFailuresTotal  *prometheus.CounterVec
	infoGauge          prometheus.Gauge
	globalRegistryLock sync.Mutex
)

// Init initializes all Prometheus metrics and registers them with the provided registry.
// This should be called once at application startup.
func Init(reg prometheus.Registerer) error {
	globalRegistryLock.Lock()
	defer globalRegistryLock.Unlock()

	// HTTP request counter: tracks all requests by method, path (normalized), and status code
	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "bunny",
			Subsystem: "proxy",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests handled by the proxy",
		},
		[]string{"method", "path", "status"},
	)
	if err := reg.Register(requestsTotal); err != nil {
		return fmt.Errorf("failed to register requestsTotal: %w", err)
	}

	// Request duration histogram: tracks latency distribution
	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "bunny",
			Subsystem: "proxy",
			Name:      "request_duration_seconds",
			Help:      "HTTP request latency in seconds",
			Buckets:   prometheus.DefBuckets, // Default buckets: .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10
		},
		[]string{"method", "path", "status"},
	)
	if err := reg.Register(requestDuration); err != nil {
		return fmt.Errorf("failed to register requestDuration: %w", err)
	}

	// Auth failures counter: tracks failed authentication attempts
	authFailuresTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "bunny",
			Subsystem: "proxy",
			Name:      "auth_failures_total",
			Help:      "Total number of authentication failures",
		},
		[]string{"reason"},
	)
	if err := reg.Register(authFailuresTotal); err != nil {
		return fmt.Errorf("failed to register authFailuresTotal: %w", err)
	}

	// Info gauge: static metric with constant label values for build info
	infoGaugeVec := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "bunny",
			Subsystem: "proxy",
			Name:      "info",
			Help:      "Proxy version and build information",
		},
		[]string{"version"},
	)
	infoGauge = infoGaugeVec.WithLabelValues("1.0.0")
	if err := reg.Register(infoGaugeVec); err != nil {
		return fmt.Errorf("failed to register infoGauge: %w", err)
	}
	infoGauge.Set(1)

	return nil
}

// RecordRequest increments the requests counter for the given method, path, and status code.
// The path should be normalized (e.g., "/dnszone/:id" instead of "/dnszone/123").
// The mutex guards the global pointer read; Prometheus operations themselves are thread-safe.
func RecordRequest(method, path, statusCode string) {
	globalRegistryLock.Lock()
	defer globalRegistryLock.Unlock()
	if requestsTotal != nil {
		requestsTotal.WithLabelValues(method, path, statusCode).Inc()
	}
}

// RecordRequestDuration records the latency for a request.
// Duration should be in seconds.
// The mutex guards the global pointer read; Prometheus operations themselves are thread-safe.
func RecordRequestDuration(method, path, statusCode string, durationSeconds float64) {
	globalRegistryLock.Lock()
	defer globalRegistryLock.Unlock()
	if requestDuration != nil {
		requestDuration.WithLabelValues(method, path, statusCode).Observe(durationSeconds)
	}
}

// RecordAuthFailure increments the auth failures counter for the given reason.
// Common reasons: "invalid_key", "permission_denied", "missing_key"
// The mutex guards the global pointer read; Prometheus operations themselves are thread-safe.
func RecordAuthFailure(reason string) {
	globalRegistryLock.Lock()
	defer globalRegistryLock.Unlock()
	if authFailuresTotal != nil {
		authFailuresTotal.WithLabelValues(reason).Inc()
	}
}

// Handler returns an HTTP handler for Prometheus metrics in text format.
// This handler should be registered at /metrics endpoint.
func Handler() http.Handler {
	return promhttp.Handler()
}

// GetMetricsText returns the Prometheus text-format output from a registry.
// This is useful for testing and debugging.
func GetMetricsText(reg prometheus.Gatherer) (string, error) {
	handler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})

	// Use httptest to capture the handler output
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body, err := io.ReadAll(w.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read metrics output: %w", err)
	}

	return string(body), nil
}
