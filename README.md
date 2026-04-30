<p align="center">
  <img src="assets/main_header.png" alt="Pastaay Logo">
  <br>
  <img src="assets/main_description.png" alt="Pastaay Description">
</p>


<p align="center">
  <img src="https://img.shields.io/badge/Release-v1.5.1-blue.svg" alt="Release">
  <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go" alt="Go Version">
</p>


## Features

* **Application-Level Chaos:** Inject faults directly into HTTP middleware, SQL drivers, gRPC Interceptors, MongoDB (v2) monitors, and Redis Hooks.
* **Smart Mode (v1.5+):** Intelligent warmup durations and automatic DDL/Setup command protection (e.g., bypassing `CREATE TABLE` or `createIndexes`) to ensure safe application boot under chaos.
* **Unbreakable Bypass Protection (v1.5.1):** Advanced SQL comment scrubbing (`/*`, `--`) prevents malicious or accidental bypasses of ignored commands.
* **Crash-Loop Immunity:** Built-in intelligent retries ensure the Chaos Engine and demo applications survive race conditions in Docker-Compose environments where databases boot slower than the API.
* **Granular & Case-Insensitive Targeting:** Target specific queries (e.g., `INSERT INTO users`) or broad protocols seamlessly. All targets are strictly evaluated via `EqualFold` to eliminate uppercase/lowercase mismatches.
* **Network-Level Sabotage:** Forcefully drop physical TCP connections. (Safeguarded to only execute on global targets to prevent accidental localized connection pool nukes).
* **Amnesia-Proof Hot-Reloading:** Watch `pastaay.yaml` via `fsnotify`. Automatically recovers from atomic file saves (Vim/Nano `Remove/Rename` events) without permanently blinding the file watcher.
* **Native Observability:** Built-in Prometheus metrics (`/metrics`) to track and graph injected faults with zero blocking latency.

---

##  Release History (Changelog)

| Version         | Highlights | Impact |
|:----------------| :--- | :--- |
| **v1.5.1**      | **Amnesia-Proof Watcher:** Fixes Linux file-save detachment bugs.<br>**Double-Chaos Shield:** Guards against Go standard library context fallbacks.<br>**Pointer-Safe Pipelines:** Corrects slice iteration memory traps in Redis Hooks.<br>**Crash-Loop Immunity:** Resilient DB dialing. | Achieves absolute structural perfection. Zero memory leaks, zero silent bypasses, and 100% accurate policy targeting in production. |
| **v1.5.0**      | **Smart Mode:** Warmup Shield & DDL Ignorer.<br>**Optimized Engine:** Map-based caching for O(1) policy checks.<br>**Network Sabotage:** TCP `drop_connection` capabilities. | Allows safe DB migrations during chaos. Delivers high-throughput performance with instant hot-reloading. |
| **v1.0 - v1.4** | HTTP Middleware, Redis Hooks, gRPC Interceptors, SQL Driver Wrapper, YAML Hot-Reloading, and Native Prometheus Metrics. | Established the core chaos engine architecture, baseline protocols, and native observability. |

<br>

---
## Documentation
Dive deep into Pastaay's mechanics using our official documentation:
* [The Configuration Guide](docs/configuration.md) - Learn how to write policies, target endpoints, and control the blast radius.
* [Architecture & Engine](docs/architecture.md) - Understand how the Policy Engine achieves zero latency lookups, and how we solved deep OS/Compiler integration bugs.
---

## Installation

```bash
go get github.com/CemAkan/pastaay
```
---

## Quick Start

1. Create a pastaay.yaml configuration file:

```YAML

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

---

## 2. Integrate into your Go application:

```Go

package main

import (
    "net/http"
    "github.com/CemAkan/pastaay/pkg/config"
    "github.com/CemAkan/pastaay/pkg/ritual"
    "github.com/CemAkan/pastaay/pkg/metrics"
)

func main() {
    // Load config & enable amnesia-proof hot-reload
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
```Bash
cd examples/demo
docker compose up -d --build
```
<br>

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

## Contributing

Contributions from the community are always welcome <3 Whether you are looking to build a new protocol interceptor, patch a core bug, or refine the documentation, your input is highly valued.
Please read the [Contributing Guide](CONTRIBUTING.md) for detailed instructions on the development workflow, core architectural guidelines (including pointer safety and interceptor fallbacks), and how to submit a Pull Request.
---

## License

Pastaay is open-sourced software licensed under the [MIT license](LICENSE).

---

<p align="center">
<img src="assets/main_bottom.png" alt="Pastaay QR Code">
</p>
