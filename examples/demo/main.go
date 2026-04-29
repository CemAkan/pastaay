package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
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

// getEnv reads an environment variable or returns a fallback value (Useful for both Docker and Local setups)
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
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
	// Use redis:6379 if running in Docker, otherwise localhost:6379
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	rdb.AddHook(redischaos.NewChaosHook(cfgManager))

	// 5. Database Setup with Pastaay SQL Chaos Wrapper
	// Use db:5432 if running in Docker, otherwise localhost:5433
	dbDSN := getEnv("DB_DSN", "postgres://pastaay:secret@localhost:5433/shortener?sslmode=disable")
	sqlchaos.Register("pastaay-postgres", &pq.Driver{}, cfgManager)
	db, err := sql.Open("pastaay-postgres", dbDSN)
	if err != nil {
		log.Fatalf("Failed to open database connection: %v", err)
	}
	defer db.Close()

	// Wait for the DB to boot up and create the table
	time.Sleep(3 * time.Second)
	_, _ = db.Exec("CREATE TABLE IF NOT EXISTS links (id SERIAL PRIMARY KEY, url TEXT)")

	// 6. Setup Application Router
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/shorten", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// CHECK CACHE FIRST (Pastaay might intercept and simulate a Cache Miss)
		val, err := rdb.Get(ctx, "latest_link").Result()

		if err == redis.Nil {
			log.Println("App: Cache Miss! Hitting database...")

			// Write to database (Pastaay might inject latency or synthetic errors here)
			_, dbErr := db.Exec("INSERT INTO links (url) VALUES ('http://short.ly/xyz123')")
			if dbErr != nil {
				log.Printf("DB Error: %v", dbErr)
				// FIX: If the database fails, abort the request and return a 500 status code!
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"message": "Internal Server Error: Database failure"}`))
				return
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
