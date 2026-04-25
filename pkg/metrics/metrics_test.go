package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// TestMetricsEndpoint ensures that the Prometheus handler correctly exposes our custom metrics.
func TestMetricsEndpoint(t *testing.T) {
	// 1. Simulate a chaos injection to increment the counter
	InjectedFaultsTotal.WithLabelValues("/api/test", "latency").Inc()

	// 2. Create a mock HTTP request to the /metrics endpoint
	req := httptest.NewRequest("GET", "/metrics", nil)
	recorder := httptest.NewRecorder()

	// 3. Serve the request using the standard Prometheus handler
	handler := promhttp.Handler()
	handler.ServeHTTP(recorder, req)

	// 4. Assert HTTP Status is 200 OK
	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200 OK, got %d", recorder.Code)
	}

	// 5. Assert our custom metric is present in the output body
	body := recorder.Body.String()
	if !strings.Contains(body, "pastaay_injected_faults_total") {
		t.Errorf("Expected metric 'pastaay_injected_faults_total' not found in response body")
	}
}
