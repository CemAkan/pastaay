package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/CemAkan/pastaay/pkg/metrics"
	"github.com/CemAkan/pastaay/pkg/ritual"
)

// Response struct for the URL shortener
type Response struct {
	Message string `json:"message"`
	Short   string `json:"short_url,omitempty"`
}

func main() {
	// 1. Load Initial Pastaay Configuration
	cfg, err := config.LoadConfig("pastaay.yaml")
	if err != nil {
		log.Fatalf("Failed to load initial config: %v", err)
	}

	// 2. Initialize Thread-Safe Configuration Manager
	cfgManager := config.NewManager(cfg)

	// 3. Enable Hot Reloading (Watch the YAML file for live changes)
	err = config.WatchConfig("pastaay.yaml", func(newCfg *config.PastaayConfig) {
		cfgManager.Update(newCfg)
		log.Println("Demo App: Pastaay configuration updated seamlessly in memory!")
	})
	if err != nil {
		log.Fatalf("Failed to start config watcher: %v", err)
	}

	// 4. Start Prometheus Metrics Server on port 2112
	go metrics.StartServer(":2112")

	// 5. Setup the Target Application (URL Shortener API)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/shorten", func(w http.ResponseWriter, r *http.Request) {
		// Mock URL Shortener logic
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Response{
			Message: "URL successfully shortened!",
			Short:   "http://short.ly/xyz123",
		})
	})

	// 6. Wrap the application router with Pastaay Chaos Middleware
	chaosHandler := ritual.Middleware(cfgManager)(mux)

	// 7. Start the main application server on port 8080
	log.Println("Demo App: URL Shortener running on :8080 with Pastaay SDK enabled")
	if err := http.ListenAndServe(":8080", chaosHandler); err != nil {
		log.Fatalf("Server crashed: %v", err)
	}
}
