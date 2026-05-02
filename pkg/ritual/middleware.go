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
			if mgr.IsCommandIgnored("http", r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			policies := mgr.GetActivePolicies("http")
			for _, p := range policies {
				if matchPath(r.URL.Path, &p) && matchHeaders(r, p.MatchHeaders) {
					metricTag := "http:" + p.Target
					if p.LatencyChance > 0 && rand.Float64() < p.LatencyChance {
						metrics.InjectedFaultsTotal.WithLabelValues(metricTag, "latency").Inc()
						timer := time.NewTimer(p.LatencyDuration)
						select {
						case <-timer.C:
							timer.Stop()
						case <-r.Context().Done():
							timer.Stop()
							w.WriteHeader(499)
							return
						}
					}
					if p.ErrorChance > 0 && rand.Float64() < p.ErrorChance {
						metrics.InjectedFaultsTotal.WithLabelValues(metricTag, "error").Inc()
						status := p.ErrorCode
						if status == 0 {
							status = http.StatusInternalServerError
						}
						w.Header().Set("Content-Type", "application/json")
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

func matchPath(reqPath string, p *config.Policy) bool {
	if strings.EqualFold(p.Target, "all") || strings.EqualFold(reqPath, p.Target) {
		return true
	}
	if p.IsWildcard {
		reqPathUpper := strings.ToUpper(reqPath)
		if strings.HasPrefix(reqPathUpper, p.WildcardPrefix) {
			remaining := reqPathUpper[len(p.WildcardPrefix):]
			if strings.HasSuffix(p.WildcardPrefix, "/") || len(remaining) == 0 || remaining[0] == '/' {
				return true
			}
		}
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
