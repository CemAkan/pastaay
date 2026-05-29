<p align="center">
  <img src="docs/assets/main_header.png" alt="Pastaay Logo">
  <br>
  <img src="docs/assets/main_description.png" alt="Pastaay Description">
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Release-v2.3.1-stable.svg" alt="Release">
  <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go" alt="Go Version">
  <img src="https://img.shields.io/badge/License-MIT-green.svg" alt="License">
</p>

## Core Architecture & Features

**The Chaos Engine**
* **Universal Interceptors:** Native fault injection for Kafka, RabbitMQ, HTTP, gRPC, SQL, MongoDB, and Redis.
* **Resource Sabotage:** Simulate CPU starvation and memory leaks with guaranteed cleanup via the Amnesia Protocol.
* **Security Hardened:** Constant time token verification, memory bounded payload processing (`io.LimitReader`), and native evasion protection.
* **Deterministic Cascading:** Complex gRPC stream rules with stable session hashing that do not short circuit.

**Control & Observability**
* **AI SRE Copilot (Oracle):** Native multi LLM integration (Gemini, Claude, GPT) that autonomously analyzes live telemetry to generate and inject optimal chaos configurations.
* **Kinetic Control Plane:** Fleet wide orchestration via the **`pastaayctl`** CLI, featuring imperative strikes, SLA guarded autopilot, and real time telemetry dashboards.
* **Distributed Tracing:** Zero allocation OpenTelemetry (OTLP) integration for visualizing chaos events across microservices without goroutine leaks.
* **Self aware Sensors:** Real time health monitoring and asynchronous telemetry for remote control providers.
* **Web Console:** Centralized dashboard with real time telemetry grid, drag and drop policy builder, and the **Resilience Probe**, an Apdex based system resilience monitor with server side proxy probing and diagnostic field popovers.

**Cloud native & GitOps**
* **Kubernetes Native:** Seamlessly manage chaos via Custom Resource Definitions (`ChaosPolicy`) powered by the Pastaay Operator.
* **GitOps Ready:** Full reference architectures for ArgoCD and Flux with autonomous rollbacks powered by the new `duration` spec.
* **CI/CD Integration:** Native GitHub Action (`pastaay-strike`) to execute SLA guarded chaos experiments directly in your pipelines.
---

## Hot reloading & Reactivity

Pastaay is built to be reactive. The engine monitors configuration states via an **amnesia proof filesystem watcher** and remote telemetry channels. It instantly transitions between stable, high latency (Glitch), and disconnected (Void) states without dropping underlying connections or requiring service restarts.

<p align="center">
  <img src="docs/assets/hot_reload_demo.gif" width="850" alt="Pastaay Hot reloading Demonstration">
</p>

---

## Distributed Tracing (OpenTelemetry)

Pastaay features zero allocation distributed tracing out-of-the-box. It automatically injects high-fidelity spans into your active context during a chaos event, providing granular visibility into exactly *where*, *when*, and *how* your system was disrupted.

To enable tracing, configure the following environment variable on your host application:

| Environment Variable | Description | Example |
| :--- | :--- | :--- |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | The gRPC endpoint of your OTel Collector. If left empty, tracing safely defaults to a zero overhead `No-Op` mode. | `http://otel-collector:4317` |
| `PASTAAY_WEBHOOK_TOKEN` | Shared secret for webhook API and web console. When set, all `/console/api/*` and `/chaos/webhook` routes require `X-Pastaay-Token` header. | `my-secret-token` |
| `PASTAAY_DEV_ALLOW_NO_TOKEN` | Set to `1` to skip token auth on protected routes. Only for local development. Never set in production. | `1` |

### The Zero overhead Guarantee
Pastaay utilizes OpenTelemetry's `BatchSpanProcessor`. This means chaos spans are flushed asynchronously. Even if your tracing backend (like Jaeger or Zipkin) goes offline, experiences severe latency, or is overwhelmed by trace volume, Pastaay will **never block your application's critical path** or leak goroutines.

---

## Zero Allocation

Pastaay is built to survive high throughput data streams. Our core evaluator guarantees **O(1)** policy lookups and **0 Bytes** of memory allocation per operation, ensuring your application never suffers from Garbage Collection (GC) spikes.

<p align="center">
  <img src="docs/assets/benchmark.png" alt="Pastaay Zero Allocation Benchmark">
</p>

---

## Evolution & Roadmap

<p align="center">
  <img src="docs/assets/milkshake4drBishop.gif" alt="FRINGE <3">
</p>

Pastaay is a continuously evolving enterprise chaos engineering suite. Our development phases are strictly focused on cloud native scalability and GitOps integrations.

| Phase              | Theme | Architecture Goals |
|:-------------------| :--- | :--- |
| **Current (v2.3)** | **Web Console** | Centralized web dashboard for direct fleet management, visual impact analysis, and interactive documentation. |
| **Next**           | **CEL driven Rule Engine** | **Dynamic Evaluation:** Integrating Google's Common Expression Language (CEL) to allow complex, AST-compiled conditional chaos rules (e.g., payload limits, header regex) with zero allocation overhead. |
| **Future**         | **Trace aware Injection** | **context propagated Chaos:** Leveraging OpenTelemetry Baggage to inject faults based on the complete distributed request journey, targeting specific end-to-end transaction flows across the fleet. |

## Web Console

Pastaay v2.3 introduces a fully client side **Web Console**, a real time observability hub served directly from the engine's embedded filesystem at `http://localhost:2112/console`. No external dependencies, no Node.js, no React.

<p align="center">
  <img src="docs/assets/web_console_demo.gif" width="850" alt="Pastaay Web Console Demo">
</p>

**Telemetry Panels**, A modular, drag and drop grid with persistent layout:

* **Global Fault Velocity:** Real time line chart of total fault injection rate (req/s). Powered by ECharts, reading `pastaay_injected_faults_total` directly from the engine's Prometheus gatherer.
* **Blast Radius Matrix:** Stacked bar chart correlating errors, latency spikes, and dropped connections across the top 5 most targeted services.
* **System Output Journal:** Lock free circular log viewer with hierarchical filtering (Pod → Protocol → Method), text search, live/pause toggle, and click to decrypt payload tracing. Streams Kubernetes pod logs via the Watch API.
* **Resilience Probe:** Apdex based health monitor probing target URLs through a **server side proxy** (`POST /console/api/probe`) to bypass CORS. Features multi target round robin, EMA smoothed scoring, adjustable thresholds, and clickable diagnostic popovers.

**Key Capabilities:**
* **Sortable & Persistent:** Drag and drop reordering saved to `localStorage`.
* **Expand/Collapse:** Each panel expands to show detailed diagnostics and tuning controls.
* **Engine Status Bar:** Live sensor fabric, active policy count, and emergency `HALT EXPERIMENTS`.
* **Dark/Light Theme:** Toggle persisted across sessions.

**Additional Views:**

* **Builder**: Visual policy configurator with type specific sabotage fields (gRPC stream modes, RAM chunks, CPU throttles). Generates YAML payloads validated by a blast radius guard.
* **Oracle**: AI SRE Copilot: paste infrastructure context and let LLMs autonomously generate optimal chaos configurations.
* **Docs**: Interactive documentation with full text search, navigation tree, and API reference.

> **Full architecture, API reference & diagnostic field docs:** [docs/web_console.md](docs/web_console.md)

---

## Documentation

Dive deep into Pastaay's mechanics using our official documentation:
* [The Configuration Guide](docs/configuration.md) - Learn how to write policies, target endpoints, and control the blast radius.
* [Architecture & Engine](docs/architecture.md) - Understand how the Policy Engine achieves zero latency lookups, and how we solved deep OS/Compiler integration constraints.
* [Remote Control & Cloud native Sensors](docs/remote_control.md) - Learn how to securely control chaos across massive fleets via Redis, Kubernetes, or Webhooks.
* [pastaayctl: Kinetic Control Plane Reference](docs/pastaayctl.md) - Master the CLI to orchestrate fleet wide chaos, run SLA guarded autopilot experiments, and monitor real time kinetic impact.
* [Pastaay Kubernetes Operator](docs/operator.md) - Learn how to deploy the operator and manage chaos natively using Kubernetes CRDs.
* [GitOps & CI/CD Integrations](examples/gitops/README.md) - Reference architectures for declarative chaos management via ArgoCD and pipeline automation.
* [Web Console](docs/web_console.md) - Explore the centralized dashboard for fleet management and impact visualization.

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
   // Load config & enable amnesia proof hot reload
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

### 3. Deploy the Operator (Optional, for Kubernetes environments):

```bash
# Register the CRD
make -C operator install

# Deploy the operator to your cluster
make -C operator deploy IMG=<your-registry>/pastaay-operator:v2.3.1
```

---

## Running the Demos

Pastaay ships with two distinct examples to help you understand both its integration mechanics and its real time reactivity.
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
   A standalone terminal user interface built to demonstrate Pastaay's amnesia proof hot reloading. This is the source of the GIF shown above.
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

Pastaay is open source software licensed under the [MIT License](LICENSE).

---

## Read the Deep Dive

[I Built a Chaos Engine That Goes Where No Tool Has Gone Before](https://medium.com/@cemakan/i-built-a-chaos-engineering-engine-that-goes-where-no-tool-has-gone-before-65d88fb141f3)

16 sections covering every interceptor, the watcher, the guard, the Oracle prompt engineering, and the production bugs I hit along the way.

---

<br>

<p align="center">
  <img src="docs/assets/main_footer.png" alt="Pastaay QR Code">
</p>


