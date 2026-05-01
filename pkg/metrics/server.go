package metrics

import (
	"log"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	serverOnce sync.Once
)

// GetHandler returns the standard Prometheus HTTP handler.
func GetHandler() http.Handler {
	return promhttp.Handler()
}

// StartServer boots up an independent HTTP server to expose Prometheus metrics.
func StartServer(port string) {
	serverOnce.Do(func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", GetHandler())

		log.Printf("Pastaay: Metrics server listening on %s/metrics\n", port)

		go func() {

			if err := http.ListenAndServe(port, mux); err != nil && err != http.ErrServerClosed {
				log.Printf("[ERROR] Pastaay: Metrics server failed on %s (Port in use?). Metrics disabled: %v\n", port, err)
			}
		}()
	})
}
