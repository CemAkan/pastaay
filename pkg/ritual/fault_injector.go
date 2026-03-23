package ritual

import (
	"math/rand"
	"net/http"
	"time"
)

type Config struct {
	LatencyChance float64
	LatencyValue  time.Duration
	ErrorChance   float64
}

func FaultInjector(cfg Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if rand.Float64() < cfg.LatencyChance {
				time.Sleep(cfg.LatencyValue)
			}
			if rand.Float64() < cfg.ErrorChance {
				http.Error(w, `Fault Injected`, 500)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
