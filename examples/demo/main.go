package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/lib/pq"
	"github.com/redis/go-redis/v9"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/CemAkan/pastaay/pkg/metrics"
	"github.com/CemAkan/pastaay/pkg/redischaos"
	"github.com/CemAkan/pastaay/pkg/ritual"
	"github.com/CemAkan/pastaay/pkg/sqlchaos"
)

type Response struct {
	Message string `json:"message"`
	Source  string `json:"source"`
	Short   string `json:"short_url,omitempty"`
}

func main() {
	// 1. Load Initial Configuration
	cfg, err := config.LoadConfig("pastaay.yaml")
	if err != nil {
		log.Fatalf("Failed to load initial config: %v", err)
	}
	cfgManager := config.NewManager(cfg)

	// 2. Enable Hot Reloading
	err = config.WatchConfig("pastaay.yaml", func(newCfg *config.PastaayConfig) {
		cfgManager.Update(newCfg)
		log.Println("Demo App: Pastaay configuration updated seamlessly!")
	})

	// 3. Start Metrics Server
	go metrics.StartServer(":2112")

	// 4. Redis Client with Pastaay Chaos Hook
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379", // Use localhost for local dev, "redis:6379" if in Docker
	})
	rdb.AddHook(redischaos.NewChaosHook(cfgManager))

	// 5. Database Setup with Pastaay SQL Chaos Wrapper
	sqlchaos.Register("pastaay-postgres", &pq.Driver{}, cfgManager)
	db, err := sql.Open("pastaay-postgres", "postgres://pastaay:secret@localhost:5432/shortener?sslmode=disable")
	if err != nil {
		log.Fatalf("Failed to open database connection: %v", err)
	}
	defer db.Close()
	time.Sleep(2 * time.Second)
	_, _ = db.Exec("CREATE TABLE IF NOT EXISTS links (id SERIAL PRIMARY KEY, url TEXT)")

	// 6. Setup Application Router
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/shorten", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// CHECK CACHE FIRST (Pastaay might intercept and simulate a Cache Miss)
		val, err := rdb.Get(ctx, "latest_link").Result()

		if err == redis.Nil {
			log.Println("App: Cache Miss! Hitting database...")
			// Write to database (Pastaay might inject latency here)
			_, dbErr := db.Exec("INSERT INTO links (url) VALUES ('http://short.ly/xyz123')")
			if dbErr != nil {
				log.Printf("DB Error: %v", dbErr)
			}
			// Write back to cache
			rdb.Set(ctx, "latest_link", "http://short.ly/xyz123", 1*time.Minute)

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(Response{Message: "Saved and Cached!", Source: "Database", Short: "http://short.ly/xyz123"})
			return
		} else if err != nil {
			log.Printf("Redis Error: %v", err)
		}

		// CACHE HIT (Returns quickly from here if Pastaay doesn't sabotage)
		log.Println("App: Cache Hit!")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{Message: "Retrieved from Cache!", Source: "Redis Cache", Short: val})
	})

	// 7. Wrap HTTP Handler with Pastaay HTTP Chaos Middleware
	chaosHandler := ritual.Middleware(cfgManager)(mux)

	log.Println("Demo App: Server running on :8080 with HTTP, SQL, and Redis Chaos enabled")
	if err := http.ListenAndServe(":8080", chaosHandler); err != nil {
		log.Fatalf("Server crashed: %v", err)
	}
}
