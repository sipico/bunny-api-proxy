package metrics

import (
	"net/http"
	"regexp"
	"time"
)

// numericSegment is a compiled regex that matches numeric path segments
// It's compiled once at package init time for efficiency
var numericSegment = regexp.MustCompile(`/(\d+)`)

// statusRecorder wraps http.ResponseWriter to capture the status code
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

// WriteHeader captures the status code and writes it to the underlying ResponseWriter
func (r *statusRecorder) WriteHeader(code int) {
	if !r.written {
		r.statusCode = code
		r.written = true
		r.ResponseWriter.WriteHeader(code)
	}
}

// Write ensures WriteHeader is called before writing body
func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.written {
		r.statusCode = http.StatusOK
		r.written = true
	}
	return r.ResponseWriter.Write(b)
}

// Middleware returns an HTTP middleware that records Prometheus metrics for each request.
// It tracks:
// - Request count by method, path, and status code
// - Request duration (latency)
// - Panics are recorded as 500 status codes
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Wrap the response writer to capture the status code
		recorder := &statusRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // default if not explicitly set
		}

		// Record start time for duration measurement
		startTime := time.Now()

		// Recover from any panics to record metrics before re-panicking
		defer func() {
			// Calculate request duration in seconds
			duration := time.Since(startTime).Seconds()

			// Get the status code (default to 500 if a panic occurred)
			statusCode := recorder.statusCode
			if statusCode == 0 {
				statusCode = http.StatusInternalServerError
			}

			// Normalize the path to avoid cardinality explosion
			// e.g., /dnszone/123 becomes /dnszone/:id
			normalizedPath := normalizePath(r.URL.Path)

			// Record metrics
			statusStr := http.StatusText(statusCode)
			if statusStr == "" {
				statusStr = "UNKNOWN"
			}

			RecordRequest(r.Method, normalizedPath, statusStr)
			RecordRequestDuration(r.Method, normalizedPath, statusStr, duration)

			// If a panic occurred, recover it temporarily to log metrics
			// Note: we don't re-panic here - let the handler deal with panic recovery
			if err := recover(); err != nil {
				// If a panic occurred, try to write 500 status if not already written
				if !recorder.written {
					recorder.statusCode = http.StatusInternalServerError
					recorder.WriteHeader(http.StatusInternalServerError)
				}
				// Don't re-panic - middleware should handle it gracefully
			}
		}()

		// Call the next handler
		next.ServeHTTP(recorder, r)
	})
}

// normalizePath takes a request path and returns a normalized version for use as a metric label.
// This prevents cardinality explosion from unique IDs in paths.
// Examples:
//
//	/dnszone/123 -> /dnszone/:id
//	/dnszone/456/records/789 -> /dnszone/:id/records/:id
func normalizePath(path string) string {
	return numericSegment.ReplaceAllString(path, "/:id")
}
