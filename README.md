<p align="center">
  <img src="assets/header.png" alt="Pastaay Logo">
  <br>
  <img src="assets/description.png" alt="Pastaay Description">
</p>


## Features

* **Application-Level Chaos:** Inject faults directly into HTTP middleware and SQL drivers.
* **Hot-Reloading Configuration:** Update chaos policies on-the-fly via a `pastaay.yaml` file without restarting your application.
* **Targeted Faults:** Apply chaos to specific HTTP paths or database layers based on probability percentages.
* **Native Observability:** Built-in Prometheus metrics (`/metrics`) to track and graph injected faults.

## Installation

```bash
go get github.com/CemAkan/pastaay
````

## Quick Start

**1. Create a `pastaay.yaml` configuration file:**

```yaml
policies:
  - type: http
    target: /api/v1/shorten
    latency_chance: 0.5
    latency_duration: 300ms
    error_chance: 0.1

  - type: sql
    target: database
    latency_chance: 1.0
    latency_duration: 200ms
```

**2. Integrate into your Go application:**

```go
package main

import (
	"net/http"
	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/CemAkan/pastaay/pkg/ritual"
	"github.com/CemAkan/pastaay/pkg/metrics"
)

func main() {
	// Load config & enable hot-reload
	cfg, _ := config.LoadConfig("pastaay.yaml")
	cfgManager := config.NewManager(cfg)
	config.WatchConfig("pastaay.yaml", cfgManager.Update)

	// Start Prometheus metrics server
	go metrics.StartServer(":2112")

	// Setup your standard router
	mux := http.NewServeMux()
	mux.HandleFunc("/api/hello", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World!"))
	})

	// Wrap with Pastaay Chaos Middleware
	chaosHandler := ritual.Middleware(cfgManager)(mux)
	http.ListenAndServe(":8080", chaosHandler)
}
```

## Running the Demo (Docker)

To see Pastaay in action with a complete URL Shortener API, PostgreSQL database, Prometheus, and Grafana:

```bash
docker compose up -d
```

* **API:** `http://localhost:8080`
* **Metrics:** `http://localhost:2112/metrics`
* **Prometheus UI:** `http://localhost:9090`
* **Grafana:** `http://localhost:3000`

-----

<p align="center">
<img src="assets/bottom.png" alt="Pastaay QR Code">
</p>
