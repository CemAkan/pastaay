package ritual

import (
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/CemAkan/pastaay/pkg/metrics"
)

// Middleware creates an HTTP handler that intercepts requests and applies chaos policies.
func Middleware(cfgManager *config.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			currentConfig := cfgManager.Get()

			var activePolicy *config.Policy
			for _, policy := range currentConfig.Policies {
				if matchPath(r.URL.Path, policy.Target) {
					activePolicy = &policy
					break
				}
			}

			if activePolicy != nil && activePolicy.Type == "http" {

				// Latency Injection
				if rand.Float64() < activePolicy.LatencyChance {
					log.Printf("Pastaay: Injecting %v latency to %s", activePolicy.LatencyDuration, r.URL.Path)

					// Prometheus sayacını gecikme (latency) etiketiyle 1 artır
					metrics.InjectedFaultsTotal.WithLabelValues(r.URL.Path, "latency").Inc()

					time.Sleep(activePolicy.LatencyDuration)
				}

				// Error Injection
				if rand.Float64() < activePolicy.ErrorChance {
					log.Printf("Pastaay: Injecting 500 Error to %s", r.URL.Path)

					// Prometheus sayacını hata (error) etiketiyle 1 artır
					metrics.InjectedFaultsTotal.WithLabelValues(r.URL.Path, "error").Inc()

					http.Error(w, "Pastaay: Ritual Fault Injected", http.StatusInternalServerError)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func matchPath(requestPath, targetPath string) bool {
	if requestPath == targetPath {
		return true
	}
	if strings.HasSuffix(targetPath, "*") {
		basePath := strings.TrimSuffix(targetPath, "*")
		return strings.HasPrefix(requestPath, basePath)
	}
	return false
}
