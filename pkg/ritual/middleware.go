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

// Middleware creates an HTTP handler that intercepts requests and applies targeted chaos policies.
func Middleware(cfgManager *config.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			currentConfig := cfgManager.Get()

			var activePolicy *config.Policy
			for _, policy := range currentConfig.Policies {
				// We only care about HTTP policies here
				if policy.Type == "http" && matchPath(r.URL.Path, policy.Target) && matchHeaders(r, policy.MatchHeaders) {
					p := policy
					activePolicy = &p
					break
				}
			}

			if activePolicy != nil {
				// Latency Injection
				if activePolicy.LatencyChance > 0 && rand.Float64() < activePolicy.LatencyChance {
					log.Printf("Pastaay HTTP: Injecting %v latency to %s", activePolicy.LatencyDuration, r.URL.Path)
					metrics.InjectedFaultsTotal.WithLabelValues(r.URL.Path, "latency").Inc()
					time.Sleep(activePolicy.LatencyDuration)
				}

				// Error Injection
				if activePolicy.ErrorChance > 0 && rand.Float64() < activePolicy.ErrorChance {
					metrics.InjectedFaultsTotal.WithLabelValues(r.URL.Path, "error").Inc()

					// Default HTTP Status Code (Return 500 if not specified)
					statusCode := activePolicy.ErrorCode
					if statusCode == 0 {
						statusCode = http.StatusInternalServerError
					}

					// Default Error Body (Return generic JSON if not specified)
					responseBody := activePolicy.ErrorBody
					if responseBody == "" {
						responseBody = `{"error": "Pastaay Chaos Injected"}`
					}

					log.Printf("Pastaay HTTP: Injecting %d error to %s", statusCode, r.URL.Path)

					// Set headers, write custom body, and abort the request
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(statusCode)
					w.Write([]byte(responseBody))
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

func matchHeaders(r *http.Request, requiredHeaders map[string]string) bool {
	for key, expectedValue := range requiredHeaders {
		if r.Header.Get(key) != expectedValue {
			return false
		}
	}
	return true
}
