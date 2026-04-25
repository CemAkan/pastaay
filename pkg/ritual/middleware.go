package ritual

import (
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
)

// FaultInjector creates an HTTP handler that intercepts requests and applies dynamic chaos policies.
func Middleware(cfgManager *config.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1. En güncel konfigürasyonu bellekten güvenle al
			currentConfig := cfgManager.Get()

			// 2. Gelen isteğin URL'si YAML'daki hedeflerle eşleşiyor mu kontrol et
			var activePolicy *config.Policy
			for _, policy := range currentConfig.Policies {
				if matchPath(r.URL.Path, policy.Target) {
					activePolicy = &policy
					break
				}
			}

			// 3. Eğer eşleşen bir kural varsa, kaosu başlat!
			if activePolicy != nil && activePolicy.Type == "http" {

				// Gecikme (Latency) Enjeksiyonu
				if rand.Float64() < activePolicy.LatencyChance {
					log.Printf("Pastaay: Injecting %v latency to %s", activePolicy.LatencyDuration, r.URL.Path)
					time.Sleep(activePolicy.LatencyDuration)
				}

				// Hata (Error) Enjeksiyonu
				if rand.Float64() < activePolicy.ErrorChance {
					log.Printf("Pastaay: Injecting 500 Error to %s", r.URL.Path)
					http.Error(w, "Pastaay: Ritual Fault Injected", http.StatusInternalServerError)
					return // Hata verdiysek uygulamaya gitmesini engelle
				}
			}

			// 4. Kaos kuralı yoksa veya ihtimaller tutmadıysa isteği uygulamaya geçir
			next.ServeHTTP(w, r)
		})
	}
}

// matchPath checks if the incoming request matches the YAML target rule.
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
