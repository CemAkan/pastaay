package config

import (
	"io"
	"log"
	"net/http"

	"gopkg.in/yaml.v3"
)

// WebhookHandler provides an HTTP endpoint for AWS FIS or CI/CD pipelines
// to dynamically inject or revoke chaos policies via POST requests.
func WebhookHandler(reloadCallback func(*PastaayConfig)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var newCfg PastaayConfig
		if err := yaml.Unmarshal(body, &newCfg); err != nil {
			log.Printf("[Pastaay-Webhook] Invalid chaos payload: %v", err)
			http.Error(w, "Invalid YAML/JSON payload structure", http.StatusBadRequest)
			return
		}

		reloadCallback(&newCfg)
		log.Printf("[Pastaay-Webhook] Engine memory hot-swapped via HTTP webhook")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok", "message":"Chaos policies applied successfully"}`))
	}
}
