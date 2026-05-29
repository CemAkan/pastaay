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

				latencyHit := p.LatencyChance > 0 && rand.Float64() < p.LatencyChance
				errorHit := p.ErrorChance > 0 && rand.Float64() < p.ErrorChance

				if latencyHit && errorHit {
					// Tie-break
					latencyHit = false
				}

				if latencyHit {
					if injectLatency(w, r, p) {
						// Client disconnected during latency
						return
					}
					// Latency injected
					continue
				}

				if errorHit {
					injectError(w, r, p)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// injectLatency blocks for the policy duration or until ctx is done.
func injectLatency(w http.ResponseWriter, r *http.Request, p *config.Policy) bool {
	metrics.InjectedFaultsTotal.WithLabelValues(p.MetricTag, "latency").Inc()
	ctx, span := tracing.StartChaosSpan(r.Context(), "pastaay.http.latency", p.Target, "latency")
	defer span.End()

	telemetry.EmitInfo("http", "HTTP Latency Injected", map[string]interface{}{
		"duration": p.LatencyDuration.String(),
		"target":   p.Target,
	}, span)

	timer := time.NewTimer(p.LatencyDuration)
	defer timer.Stop()

	select {
	case <-timer.C:
		return false
	case <-ctx.Done():
		// Nginx-style "client closed request" status.
		w.WriteHeader(499)
		return true
	}
}

func injectError(w http.ResponseWriter, r *http.Request, p *config.Policy) {
	metrics.InjectedFaultsTotal.WithLabelValues(p.MetricTag, "error").Inc()
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
}

// matchPath checks whether reqPath is covered by the policy.
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
