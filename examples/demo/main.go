package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/CemAkan/pastaay/pkg/metrics"
	"github.com/CemAkan/pastaay/pkg/ritual"
	"github.com/CemAkan/pastaay/pkg/sqlchaos"
	"github.com/lib/pq"
)

type Response struct {
	Message string `json:"message"`
	Short   string `json:"short_url,omitempty"`
}

func main() {
	// 1. Load Initial Configuration
	cfg, err := config.LoadConfig("../../pastaay.yaml")
	if err != nil {
		log.Fatalf("Failed to load initial config: %v", err)
	}

	cfgManager := config.NewManager(cfg)

	// 2. Enable Hot Reloading
	err = config.WatchConfig("../../pastaay.yaml", func(newCfg *config.PastaayConfig) {
		cfgManager.Update(newCfg)
		log.Println("Demo App: Pastaay configuration updated seamlessly!")
	})
	if err != nil {
		log.Fatalf("Failed to start config watcher: %v", err)
	}

	// 3. Start Metrics Server
	go metrics.StartServer(":2112")

	// 4. Register and Connect to Database using Pastaay SQL Chaos Wrapper
	sqlchaos.Register("pastaay-postgres", &pq.Driver{}, cfgManager)

	db, err := sql.Open("pastaay-postgres", "postgres://pastaay:secret@db:5432/shortener?sslmode=disable")
	if err != nil {
		log.Fatalf("Failed to open database connection: %v", err)
	}
	defer db.Close()

	// Wait for DB to be ready and create a dummy table
	time.Sleep(2 * time.Second)
	_, _ = db.Exec("CREATE TABLE IF NOT EXISTS links (id SERIAL PRIMARY KEY, url TEXT)")

	// 5. Setup Application Router
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/shorten", func(w http.ResponseWriter, r *http.Request) {
		// Execute a database query targeted by chaos policies
		_, dbErr := db.Exec("INSERT INTO links (url) VALUES ('http://short.ly/xyz123')")
		if dbErr != nil {
			log.Printf("DB Error: %v", dbErr)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Response{
			Message: "URL successfully shortened and saved to database!",
			Short:   "http://short.ly/xyz123",
		})
	})

	// 6. Wrap HTTP Handler with Pastaay HTTP Chaos Middleware
	chaosHandler := ritual.Middleware(cfgManager)(mux)

	log.Println("Demo App: URL Shortener running on :8080 with HTTP and SQL Chaos enabled")
	if err := http.ListenAndServe(":8080", chaosHandler); err != nil {
		log.Fatalf("Server crashed: %v", err)
	}
}
