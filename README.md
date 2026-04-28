<p align="center">
  <img src="assets/main_header.png" alt="Pastaay Logo">
  <br>
  <img src="assets/main_description.png" alt="Pastaay Description">
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Release-v1.3.0-blue.svg" alt="Release">
  <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go" alt="Go Version">
</p>


## Features

* **Application-Level Chaos:** Inject faults directly into HTTP middleware, SQL drivers, gRPC Interceptors, and **Redis Hooks**.
* **Blast Radius Control (Targeted Chaos):** Apply chaos exclusively to specific users or segments by matching HTTP/gRPC headers.
* **Hot-Reloading Configuration:** Update chaos policies on-the-fly via a `pastaay.yaml` file without restarting your application.
* **Native Observability:** Built-in Prometheus metrics (`/metrics`) to track and graph injected faults.
---

## Installation

```bash
go get github.com/CemAkan/pastaay
```
---

## Quick Start

**1. Create a `pastaay.yaml` configuration file:**

### Configuration (pastaay.yaml):

Pastaay uses a policy-based configuration. You can define multiple chaos rules and target specific endpoints or headers.

**For a complete list of all supported types (`http`, `sql`, `grpc`,`redis`) and parameters, please read the [Detailed Configuration Reference](docs/configuration.md).**


```yaml
version: 1
policies:
  - name: "slow-down-http"
    target: "/api/hello"
    type: "http"
    latency_chance: 1.0
    latency_duration: "2s"

  - name: "redis-cache-miss"
    target: "get"
    type: "redis"
    error_chance: 0.5 # 50% chance to simulate a cache miss (returns redis.Nil)
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
---

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
<img src="assets/main_bottom.png" alt="Pastaay QR Code">
</p>
