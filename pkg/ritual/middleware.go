package ritual

import (
	"io"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/CemAkan/pastaay/pkg/metrics"
	"github.com/CemAkan/pastaay/pkg/telemetry"
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
			for i := range policies {
				p := &policies[i]
				if !matchPath(r.URL.Path, p) || !matchHeaders(r, p.MatchHeaders) {
					continue
				}

				metricTag := p.MetricTag

				if p.LatencyChance > 0 && rand.Float64() < p.LatencyChance {
					metrics.InjectedFaultsTotal.WithLabelValues(metricTag, "latency").Inc()
					ctx, span := tracing.StartChaosSpan(r.Context(), "pastaay.http.latency", p.Target, "latency")

					telemetry.EmitInfo("http", "HTTP Latency Injected", map[string]interface{}{"duration": p.LatencyDuration.String(), "target": p.Target}, span)

					timer := time.NewTimer(p.LatencyDuration)
					select {
					case <-timer.C:
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
					defer span.End()

					status := p.ErrorCode
					if status == 0 {
						status = http.StatusInternalServerError
					}

					telemetry.EmitError("http", p.Target, "HTTP Fault Injected", p.ErrorBody, span)

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(status)

					if p.ErrorBody != "" {
						_, _ = io.WriteString(w, p.ErrorBody)
					}
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// matchPath checks whether reqPath is covered by the policy, handling exact matches, wildcards (target ending with *), and the "all" sentinel.
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
