package metrics

import (
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// StartServer boots up an independent HTTP server to expose Prometheus metrics.
// We run this on a separate port (e.g., ":2112") so it doesn't conflict with the main app.
func StartServer(port string) {
	mux := http.NewServeMux()

	// Bind the standard Prometheus scrape handler to the /metrics route
	mux.Handle("/metrics", promhttp.Handler())

	log.Printf("Pastaay: Metrics server listening on %s/metrics\n", port)

	// Start the HTTP server
	if err := http.ListenAndServe(port, mux); err != nil && err != http.ErrServerClosed {
		log.Printf("[ERROR] Pastaay: Failed to start metrics server on %s (Port in use?). Metrics disabled: %v\n", port, err)
	}
}
