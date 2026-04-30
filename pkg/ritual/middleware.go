package ritual

import (
	"math/rand/v2"
	"net/http"
	"strings"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/CemAkan/pastaay/pkg/metrics"
)

func Middleware(mgr *config.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			policies := mgr.GetActivePolicies("http")

			for _, p := range policies {
				if matchPath(r.URL.Path, p.Target) && matchHeaders(r, p.MatchHeaders) {

					if p.LatencyChance > 0 && rand.Float64() < p.LatencyChance {
						metrics.InjectedFaultsTotal.WithLabelValues(p.Target, "latency").Inc()

						timer := time.NewTimer(p.LatencyDuration)
						select {
						case <-timer.C:
						case <-r.Context().Done():
							timer.Stop()
							return
						}
						timer.Stop()
					}

					if p.ErrorChance > 0 && rand.Float64() < p.ErrorChance {
						metrics.InjectedFaultsTotal.WithLabelValues(p.Target, "error").Inc()
						status := p.ErrorCode
						if status == 0 {
							status = http.StatusInternalServerError
						}

						w.WriteHeader(status)
						w.Write([]byte(p.ErrorBody))
						return
					}

				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func matchPath(reqPath, targetPath string) bool {
	if strings.EqualFold(targetPath, "all") {
		return true
	}
	if reqPath == targetPath {
		return true
	}
	if strings.HasSuffix(targetPath, "*") {
		return strings.HasPrefix(reqPath, strings.TrimSuffix(targetPath, "*"))
	}
	return false
}

func matchHeaders(r *http.Request, required map[string]string) bool {
	for k, v := range required {
		if r.Header.Get(k) != v {
			return false
		}
	}
	return true
}
