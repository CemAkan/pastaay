<p align="center">
  <img src="assets/main_header.png" alt="Pastaay Logo">
  <br>
  <img src="assets/main_description.png" alt="Pastaay Description">
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Release-v1.5.0-blue.svg" alt="Release">
  <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go" alt="Go Version">
</p>


## Features

* **Application-Level Chaos:** Inject faults directly into HTTP middleware, SQL drivers, gRPC Interceptors, MongoDB (v2) monitors, and Redis Hooks.
* **Smart Mode (v1.5+):** Intelligent warmup durations and automatic DDL/Setup command protection (e.g., bypassing `CREATE TABLE` or `createIndexes`) to ensure safe application boot under chaos.
* **Network-Level Sabotage:** Forcefully drop physical TCP connections for databases and caches.
* **Flexible Fault Injection:** Return custom HTTP Status Codes, JSON bodies, simulate cache misses (`redis.Nil`), or inject synthetic database latency.
* **Blast Radius Control:** Apply chaos exclusively to specific users or segments by matching HTTP/gRPC headers.
* **Hot-Reloading Engine:** Update chaos policies on-the-fly via a `pastaay.yaml` file without restarting.
* **Native Observability:** Built-in Prometheus metrics (`/metrics`) to track and graph injected faults.

---

##  Release History (Changelog)

| Version | Highlights | Impact |
| :--- | :--- | :--- |
| **v1.5 (Latest)** | **Smart Mode:** Warmup Shield & DDL Ignorer.<br>**Optimized Engine:** Map-based caching for zero-latency policy checks.<br>**Mongo v2:** Full native driver support.<br>**Network Sabotage:** Physical TCP `drop_connection` capabilities. | Allows safe DB migrations during chaos. Delivers high-throughput performance with instant hot-reloading. |
| **v1.0 - v1.4** | HTTP Middleware, Redis Hooks, gRPC Interceptors, SQL Driver Wrapper, YAML Hot-Reloading, and Native Prometheus Metrics. | Established the core chaos engine architecture, baseline protocols, and native observability. |

<br>

---
## Documentation
Dive deep into Pastaay's mechanics using our official documentation:
* [The Configuration Guide](docs/configuration.md) - Learn how to write policies, target endpoints, and control the blast radius.
* [Architecture & Engine](docs/architecture.md) - Understand how the O(1) Policy Engine achieves zero-latency lookups.
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

**For a complete list of all supported types (`http`, `sql`, `grpc`,`redis`,`mongo`) and parameters, please read the [Detailed Configuration Reference](docs/configuration.md).**


```yaml
version: 1
warmup_duration: "10s"
enable_default_ignored: true

policies:
  - name: "custom-http-failure"
    target: "/api/hello"
    type: "http"
    error_chance: 1.0
    error_code: 429
    error_body: '{"error": "Pastaay Chaos: Rate Limit Exceeded"}'

  - name: "mongo-kill-switch"
    target: "all"
    type: "mongo"
    drop_connection: true
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

To see Pastaay in action with a complete URL Shortener API, PostgreSQL database, Redis, Prometheus, and Grafana:
```bash
docker compose up -d --build
```

* **API:** `http://localhost:8080`
* **Metrics:** `http://localhost:2112/metrics`
* **Prometheus UI:** `http://localhost:9090`
* **Grafana:** `http://localhost:3000`

-----

## Roadmap: The Future of Pastaay

Pastaay is rapidly evolving into a full-fledged enterprise chaos engineering suite. Here is our aggressive roadmap for the upcoming major releases:

| Version | Planned Features | Status        |
| :--- | :--- |:--------------|
| **v1.6** | **Message Brokers:** Kafka & RabbitMQ Interceptors for message queue chaos and event dropping. |  In Progress  |
| **v1.7** | **Resource Sabotage:** CPU Stressors and RAM Bloaters to simulate memory leaks and compute starvation. |  Planned    |
| **v1.8** | **Advanced Observability:** Distributed Tracing (OpenTelemetry) integration and latency percentile graphing. |  Planned    |
| **v1.9** | **Cloud & Low-Level:** AWS Fault Injection Simulator (FIS) hooking and eBPF-based packet dropping without code changes. |  Conceptual |
| **v2.0** | **The Enterprise Suite:** Kubernetes Operator (`pastaay-operator` via CRDs), CLI Tool (`pastaay-cli`), and a real-time Web Dashboard UI. |  Conceptual |

<br>

---

## License

Pastaay is open-sourced software licensed under the [MIT license](LICENSE).

---

<p align="center">
<img src="assets/main_bottom.png" alt="Pastaay QR Code">
</p>

