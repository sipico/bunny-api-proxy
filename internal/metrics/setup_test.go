package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestMain(m *testing.M) {
	// Initialize metrics with a test registry once before all tests run
	// This ensures the global variables are set up before any parallel tests access them
	testRegistry := prometheus.NewRegistry()
	Init(testRegistry)

	// Run all tests
	m.Run()
}
