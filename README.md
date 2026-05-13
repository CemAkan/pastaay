<p align="center">
  <img src="assets/main_header.png" alt="Pastaay Logo">
  <br>
  <img src="assets/main_description.png" alt="Pastaay Description">
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Release-v1.9.0-blue.svg" alt="Release">
  <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go" alt="Go Version">
  <img src="https://img.shields.io/badge/License-MIT-green.svg" alt="License">
</p>

## Core Architecture & Features

* **Universal Chaos:** Native interceptors for Kafka, RabbitMQ, HTTP, gRPC, SQL, MongoDB, and Redis.
* **Security Hardened:** Constant-time token verification (mitigates timing attacks), memory-bounded payload processing (`io.LimitReader`), and native evasion protection for SQL/HTTP boundaries.
* **Deterministic Cascading:** Complex gRPC stream rules with stable session hashing that do not short-circuit.
* **Self-Aware Sensors:** Real-time health monitoring and asynchronous telemetry for all remote control providers (K8s, Redis, Webhooks).
* **Resource Sabotage:** Simulate CPU starvation and memory leaks with guaranteed cleanup via the Amnesia Protocol.
* **Distributed Tracing:** Zero-allocation OpenTelemetry (OTLP) integration for visualizing chaos events across microservices without goroutine leaks.

---

## Hot-Reloading & Reactivity

Pastaay is built to be reactive. The engine monitors configuration states via an **amnesia-proof filesystem watcher** and remote telemetry channels. It instantly transitions between stable, high-latency (Glitch), and disconnected (Void) states without dropping underlying connections or requiring service restarts.

<p align="center">
  <img src="assets/hot_reload_demo.gif" width="850" alt="Pastaay Hot-Reloading Demonstration">
</p>

---

## Distributed Tracing (OpenTelemetry)

Pastaay features zero-allocation distributed tracing out-of-the-box. It automatically injects high-fidelity spans into your active context during a chaos event, providing granular visibility into exactly *where*, *when*, and *how* your system was disrupted.

To enable tracing, configure the following environment variable on your host application:

| Environment Variable | Description | Example |
| :--- | :--- | :--- |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | The gRPC endpoint of your OTel Collector. If left empty, tracing safely defaults to a zero-overhead `No-Op` mode. | `http://otel-collector:4317` |

### The Zero-Overhead Guarantee
Pastaay utilizes OpenTelemetry's `BatchSpanProcessor`. This means chaos spans are flushed asynchronously. Even if your tracing backend (like Jaeger or Zipkin) goes offline, experiences severe latency, or is overwhelmed by trace volume, Pastaay will **never block your application's critical path** or leak goroutines.

---

## Zero Allocation

Pastaay is built to survive high-throughput data streams. Our core evaluator guarantees **O(1)** policy lookups and **0 Bytes** of memory allocation per operation, ensuring your application never suffers from Garbage Collection (GC) spikes.

<p align="center">
  <img src="assets/benchmark.png" alt="Pastaay Zero Allocation Benchmark">
</p>

---

## Evolution & Roadmap

Pastaay is a continuously evolving enterprise chaos engineering suite. Our development phases are strictly focused on cloud-native scalability and GitOps integrations.

| Phase | Theme | Architecture Goals |
| :--- | :--- | :--- |
| **Current** | **The Cloud-Native Bridge** | Remote Control Sensors (K8s ConfigMap Watchers, Redis PubSub listeners, AWS FIS webhooks). Multi-phase validation guards and real-time health telemetry. |
| **Next** | **The Command Center** | Centralized Ecosystem: Kubernetes Operator (`pastaay-operator` via CRDs) and the **`pastaayctl`** CLI tool for fleet-wide chaos management. |
| **Future** | **GitOps & Autopilot** | Native GitHub Actions/GitLab CI integrations for Chaos-as-Code. ML-driven blast radius adjustments based on baseline traffic analysis. |

---

## Documentation

Dive deep into Pastaay's mechanics using our official documentation:
* [The Configuration Guide](docs/configuration.md) - Learn how to write policies, target endpoints, and control the blast radius.
* [Architecture & Engine](docs/architecture.md) - Understand how the Policy Engine achieves zero-latency lookups, and how we solved deep OS/Compiler integration constraints.
* [Remote Control & Cloud-Native Sensors](docs/remote_control.md) - Learn how to securely control chaos across massive fleets via Redis, Kubernetes, or Webhooks.

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

```bash
cd examples/visualizer
   # Note: Use 'run' instead of 'up' to ensure a clean TTY for the visualizer
   docker compose run --rm --service-ports app
```
<br>

---

## Contributing

Contributions from the community are always welcome❤️ Whether you are looking to build a new protocol interceptor, patch a core bug, or refine the documentation, your input is highly valued.

Please read the [Contributing Guide](CONTRIBUTING.md) for detailed instructions on the development workflow, core architectural guidelines (including pointer safety and interceptor fallbacks), and how to submit a Pull Request.

---

##  License

Pastaay is open-sourced software licensed under the [MIT License](LICENSE).

---

<p align="center">
  <img src="assets/main_bottom.png" alt="Pastaay QR Code">
</p>

