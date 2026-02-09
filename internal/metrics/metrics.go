// Package metrics provides Prometheus metrics collection for the proxy.
package metrics

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Global metrics - used by the application
	// Using atomic.Pointer for lock-free initialization checks on hot path metrics.
	requestsTotal     atomic.Pointer[prometheus.CounterVec]
	requestDuration   atomic.Pointer[prometheus.HistogramVec]
	authFailuresTotal atomic.Pointer[prometheus.CounterVec]
)

// Init initializes all Prometheus metrics and registers them with the provided registry.
// This should be called once at application startup.
func Init(reg prometheus.Registerer) error {
	// HTTP request counter: tracks all requests by method, path (normalized), and status code
	requestsTotalVec := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "bunny",
			Subsystem: "proxy",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests handled by the proxy",
		},
		[]string{"method", "path", "status"},
	)
	if err := reg.Register(requestsTotalVec); err != nil {
		return fmt.Errorf("failed to register requestsTotal: %w", err)
	}

	// Request duration histogram: tracks latency distribution
	requestDurationVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "bunny",
			Subsystem: "proxy",
			Name:      "request_duration_seconds",
			Help:      "HTTP request latency in seconds",
			Buckets:   prometheus.DefBuckets, // Default buckets: .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10
		},
		[]string{"method", "path", "status"},
	)
	if err := reg.Register(requestDurationVec); err != nil {
		return fmt.Errorf("failed to register requestDuration: %w", err)
	}

	// Auth failures counter: tracks failed authentication attempts
	authFailuresTotalVec := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "bunny",
			Subsystem: "proxy",
			Name:      "auth_failures_total",
			Help:      "Total number of authentication failures",
		},
		[]string{"reason"},
	)
	if err := reg.Register(authFailuresTotalVec); err != nil {
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
	infoGaugeInstance := infoGaugeVec.WithLabelValues("1.0.0")
	if err := reg.Register(infoGaugeVec); err != nil {
		return fmt.Errorf("failed to register infoGauge: %w", err)
	}
	infoGaugeInstance.Set(1)

	// Store metrics in atomics for lock-free access in record functions
	requestsTotal.Store(requestsTotalVec)
	requestDuration.Store(requestDurationVec)
	authFailuresTotal.Store(authFailuresTotalVec)

	return nil
}

// RecordRequest increments the requests counter for the given method, path, and status code.
// The path should be normalized (e.g., "/dnszone/:id" instead of "/dnszone/123").
// Uses atomic.Pointer for lock-free nil checks; Prometheus operations themselves are thread-safe.
func RecordRequest(method, path, statusCode string) {
	if counter := requestsTotal.Load(); counter != nil {
		counter.WithLabelValues(method, path, statusCode).Inc()
	}
}

// RecordRequestDuration records the latency for a request.
// Duration should be in seconds.
// Uses atomic.Pointer for lock-free nil checks; Prometheus operations themselves are thread-safe.
func RecordRequestDuration(method, path, statusCode string, durationSeconds float64) {
	if histogram := requestDuration.Load(); histogram != nil {
		histogram.WithLabelValues(method, path, statusCode).Observe(durationSeconds)
	}
}

// RecordAuthFailure increments the auth failures counter for the given reason.
// Common reasons: "invalid_key", "permission_denied", "missing_key"
// Uses atomic.Pointer for lock-free nil checks; Prometheus operations themselves are thread-safe.
func RecordAuthFailure(reason string) {
	if counter := authFailuresTotal.Load(); counter != nil {
		counter.WithLabelValues(reason).Inc()
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
