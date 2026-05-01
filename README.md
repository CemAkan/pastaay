<p align="center">
  <img src="assets/main_header.png" alt="Pastaay Logo">
  <br>
  <img src="assets/main_description.png" alt="Pastaay Description">
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Release-v1.6-blue.svg" alt="Release">
  <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go" alt="Go Version">
  <img src="https://img.shields.io/badge/License-MIT-green.svg" alt="License">
</p>

## Core Features

* **Universal Chaos:** Natively inject faults into Kafka, RabbitMQ, HTTP, gRPC, SQL, MongoDB, and Redis without changing your core application logic.
* **Zero-Allocation Engine:** Policy evaluator with a thread-safe architecture. Built to survive high-throughput data streams without GC spikes.
* **Smart & Surgical:** Target exact queries, topics, or routes (Case-Insensitive). Automatically protects database migrations (DDL) and container boot sequences.
* **Amnesia-Proof Hot-Reloading:** Modify `pastaay.yaml` on the fly. Zero downtime, completely immune to Linux file-replace detachment.

---

## Hot-Reloading

Pastaay is built to be reactive. The following demonstration shows the engine's **amnesia-proof hot-reload** capability via our built-in TUI visualizer. Watch as it detects manual updates to `pastaay.yaml` and instantly transitions between stable, high-latency (Glitch), and disconnected (Void) states without dropping the underlying connection or requiring a service restart.

<p align="center">
  <img src="assets/hot_reload_demo.gif" width="850" alt="Pastaay Hot-Reloading Demonstration">
</p>

> **Reactivity:** The engine reacts to `latency_chance` and `error_chance` updates within milliseconds of the file being saved.

---

## Zero Allocation 

Pastaay is built to survive high-throughput data streams. Our core evaluator guarantees **O(1)** policy lookups and **0 Bytes** of memory allocation per operation, ensuring your application never suffers from Garbage Collection (GC) spikes.

<p align="center">
  <img src="assets/benchmark.png" alt="Pastaay Zero Allocation Benchmark">
</p>

---


## Changelog

| Version | Highlights | Impact |
| :--- | :--- | :--- |
| **v1.6.0** | **Message Brokers:** Kafka & RabbitMQ Interceptors for message queue chaos and event dropping. | Delivers zero-allocation message broker chaos for high-throughput distributed systems without GC spikes. |
| **v1.5.x** | **Smart Mode:** Warmup Shield & DDL Ignorer.<br>**Amnesia-Proof Watcher:** Fixes Linux file-save detachment bugs.<br>**Double-Chaos Shield:** Guards against Go standard library fallbacks.<br>**Network Sabotage:** TCP `drop_connection`. | Achieves absolute structural perfection. Zero memory leaks, zero silent bypasses, and 100% accurate policy targeting in production. |
| **v1.0 - v1.4** | HTTP Middleware, Redis Hooks, gRPC Interceptors, SQL Driver Wrapper, YAML Hot-Reloading, and Native Metrics. | Established the core chaos engine architecture, baseline protocols, and native observability. |

<br>

---

## Documentation

Dive deep into Pastaay's mechanics using our official documentation:
* [The Configuration Guide](docs/configuration.md) - Learn how to write policies, target endpoints, and control the blast radius.
* [Architecture & Engine](docs/architecture.md) - Understand how the Policy Engine achieves zero-latency lookups, and how we solved deep OS/Compiler integration bugs.

---

##  Installation

```bash
go get github.com/CemAkan/pastaay
```

---

## Quick Start

### 1. Create a Configuration File (`pastaay.yaml`):

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

### 2. Integrate into your Go application:

```go
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

## Running the Demos

Pastaay ships with two distinct examples to help you understand both its integration mechanics and its real-time reactivity.
1. The Integration Demo
   A complete, hardened microservice stack (URL Shortener API, PostgreSQL, Redis, MongoDB, Kafka, RabbitMQ) showing how to securely integrate Pastaay without race conditions.

```bash
   cd examples/demo
   docker compose up -d --build
   docker compose logs -f app
```


* **API:** `http://localhost:8080`
* **Metrics:** `http://localhost:2112/metrics`
* **Prometheus UI:** `http://localhost:9090`
* **Grafana:** `http://localhost:3000`

<br>

2. The TUI Visualizer (Vortex)
   A standalone terminal user interface built to demonstrate Pastaay's amnesia-proof hot-reloading. This is the source of the GIF shown above.
   cd examples/visualizer

> **Note**: Use 'run' instead of 'up' to ensure a clean TTY for the visualizer
docker compose run --rm --service-ports app

---

## Roadmap:

Pastaay is rapidly evolving into a full-fledged enterprise chaos engineering suite. Here is our aggressive roadmap for the upcoming major releases:

| Version | Planned Features | Status       |
| :--- | :--- |:-------------|
| **v1.7** | **Resource Sabotage:** CPU Stressors and RAM Bloaters to simulate memory leaks and compute starvation. |  In Progress |
| **v1.8** | **Advanced Observability:** Distributed Tracing (OpenTelemetry) integration and latency percentile graphing. | Planned      |
| **v1.9** | **Cloud & Low-Level:** AWS Fault Injection Simulator (FIS) hooking and eBPF-based packet dropping without code changes. | Conceptual   |
| **v2.0** | **The Enterprise Suite:** Kubernetes Operator (`pastaay-operator` via CRDs), CLI Tool (`pastaay-cli`), and a real-time Web Dashboard UI. | Conceptual   |

<br>

---

##  Contributing

Contributions from the community are always welcome <3 Whether you are looking to build a new protocol interceptor, patch a core bug, or refine the documentation, your input is highly valued.

Please read the [Contributing Guide](CONTRIBUTING.md) for detailed instructions on the development workflow, core architectural guidelines (including pointer safety and interceptor fallbacks), and how to submit a Pull Request.

---

##  License

Pastaay is open-sourced software licensed under the [MIT License](LICENSE).

---

<p align="center">
  <img src="assets/main_bottom.png" alt="Pastaay QR Code">
</p>

