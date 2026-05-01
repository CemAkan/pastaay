package metrics

import (
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// GetHandler returns the standard Prometheus HTTP handler.
func GetHandler() http.Handler {
	return promhttp.Handler()
}

// StartServer boots up an independent HTTP server to expose Prometheus metrics.
func StartServer(port string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", GetHandler())

	log.Printf("Pastaay: Metrics server listening on %s/metrics\n", port)

	// If the port is busy, the host app keeps running, only metrics stay disabled.
	go func() {
		if err := http.ListenAndServe(port, mux); err != nil && err != http.ErrServerClosed {
			log.Printf("[ERROR] Pastaay: Metrics server failed on %s (Port in use?). Metrics disabled: %v\n", port, err)
		}
	}()
}
