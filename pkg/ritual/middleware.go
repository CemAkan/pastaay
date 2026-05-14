package ritual

import (
	"io"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/CemAkan/pastaay/pkg/metrics"
	"github.com/CemAkan/pastaay/pkg/tracing"
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

					metricTag := p.MetricTag

					if p.LatencyChance > 0 && rand.Float64() < p.LatencyChance {
						metrics.InjectedFaultsTotal.WithLabelValues(metricTag, "latency").Inc()
						ctx, span := tracing.StartChaosSpan(r.Context(), "pastaay.http.latency", p.Target, "latency")

						timer := time.NewTimer(p.LatencyDuration)
						select {
						case <-timer.C:
							timer.Stop()
							span.End()
						case <-ctx.Done():
							timer.Stop()
							span.End()
							w.WriteHeader(499)
							return
						}
					}
					if p.ErrorChance > 0 && rand.Float64() < p.ErrorChance {
						metrics.InjectedFaultsTotal.WithLabelValues(metricTag, "error").Inc()
						_, span := tracing.StartChaosSpan(r.Context(), "pastaay.http.error", p.Target, "error")

						status := p.ErrorCode
						if status == 0 {
							status = http.StatusInternalServerError
						}
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(status)
						
						io.WriteString(w, p.ErrorBody)

						span.End()
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
		if !strings.EqualFold(r.Header.Get(k), v) {
			return false
		}
	}
	return true
}
