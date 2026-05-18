<p align="center">
  <img src="assets/ctl_header.gif" alt="CLI Header"/>
</p>

**pastaayctl** is the central orchestrating command-line interface for the Pastaay Chaos Engine ecosystem. Engineered for high-scale fleet management in distributed architectures, it provides a unified platform for imperative fault injection, SLA-guarded autonomous experimentation, and real-time kinetic observability.

---

## Global Flags

All commands support the following global flags to ensure seamless CI/CD pipeline integration and programmatic parsing:

| Flag | Shorthand | Description |
| --- | --- | --- |
| `--target` | `-t` | Overrides the active profile to target a specific engine URL. |
| `--token` | `-k` | Provides the authentication token for remote webhooks. |
| `--profile` | `-p` | Temporarily sets the execution context (e.g., `prod`, `staging`). |
| `--json` | | Outputs the command result in raw JSON format for `jq` parsing. |

---

## Command Taxonomy

The CLI architecture is partitioned into five functional domains to maintain strict operational isolation between offensive operations, safety analysis, and environmental management.

### 1. Attack Vectors 

Commands designed to transition the engine's runtime state from stable to chaotic by manipulating in-memory policy pointers.

| Command | Operational Mode | Execution Logic | Technical Detail |
| --- | --- | --- | --- |
| **strike** | Imperative | Flag-based | Instant injection via terminal flags without the need for YAML boilerplate. |
| **inject** | Declarative | Payload-driven | Applies complex, multi-policy YAML/JSON configurations to a target node. |
| **snipe** | Interactive | Wizard-driven | Step-by-step guided mode for precision targeting of specific service endpoints. |
| **broadcast** | Distributed | PubSub | Disseminates policies to the entire fleet via Redis with <10ms propagation latency. |
| **rollback** | Emergency | Atomic Reset | Zeroes out all active in-memory policies to restore the system to a "safe-state." |

**Dead Man's Switch (TTL):** All attack vectors support the `--ttl` flag. This initiates a local safety heartbeat; if the operator's session terminates or the timer expires, an automatic `rollback` is triggered to prevent permanent system degradation.

---

### 2. Guard & Strategy

A static analysis layer that validates payloads before they are committed to the engine's atomic memory pointers.

| Sub-command | Analysis Type | Safety Enforcement | Impact |
| --- | --- | --- | --- |
| **lint** | Logic | Conflict Detection | Identifies overlapping targets; rejects latencies >10s and RAM chunks >4GB. |
| **plan** | Forecast | Weighted Risk Scoring | Calculates impact via $\text{Risk Score} = (E \times 0.7) + (L \times 0.3)$ and predicts span throughput. |
| **validate** | Integrity | Schema Guard | Ensures structural compliance with the Pastaay V1 specification. |

---

### 3. Resilience & Automation 

High-level automation that wraps chaos injections in active feedback loops to ensure host system survival.

| Sub-command | Feedback Loop | Scaling Logic | Termination Criteria |
| --- | --- | --- | --- |
| **run** | Passive Probes | Time-bounded | Aborts if `--health-url` becomes unreachable or if latency bounds are exceeded. |
| **autopilot** | Active Scaling | Adaptive Step | Automatically searches for the system's breaking point by ramping failure probability in 5% increments. |

**SLA Breach Criteria:** The automation engine triggers an immediate atomic rollback if probe latency exceeds `--max-latency` (default 500ms) or if the health endpoint returns an HTTP `5xx` status code.

---

### 4. Fleet Observability

Interacts with Prometheus metrics and memory export handlers to provide a real-time window into fleet topology and impact.

| Sub-command | Data Source | Visualization | Purpose |
| --- | --- | --- | --- |
| **top** | `/metrics` | Kinetic TUI | Visualizes real-time fault hits and requests-per-second (req/s) rates across the fleet. |
| **discover** | Label Stream | Topology Map | Automatically maps injectable endpoints and queries by scanning active metric labels. |
| **status** | `SensorStatus` | Matrix View | Reports the connectivity and health status of K8s, Redis, and Webhook sensors. |
| **inspect** | `/chaos/export` | Memory X-Ray | Downloads and displays raw, active chaos policies directly from the engine's memory. |

---

### 5. Management & Compliance

Auxiliary tools for configuration management, audit trails, and policy generation.

| Component | Function | Persistence | Key Feature |
| --- | --- | --- | --- |
| **Profile Registry** | Context Mgmt | `~/.pastaayctl.json` | Facilitates rapid context-switching between Prod, Staging, and Local environments. |
| **Audit History** | Event Logging | `~/.pastaay_history.json` | Records every chaos injection at the metadata level for historical compliance. |
| **Post-Mortem** | Reporting | Markdown Output | Generates a professional incident review report from the most recent chaos event. |
| **Utility** | Generation | Stdout | Produces blueprints for common scenarios such as `db-outage` or `cache-stampede`. |

### 6. Oracle

<p align="center">
  <img src="assets/oracle_banner.gif" alt="Oracle Banner"/>
</p>

Pastaay includes a native, zero-dependency Multi-LLM client that acts as an autonomous Site Reliability Engineer. The Oracle command connects to your active engine, reads live `/metrics` (kinetic impact) and health baseline latency, and passes this context to an AI provider.

| Command | Provider Engine | Key Feature | Purpose |
| --- | --- | --- | --- |
| **oracle** | Gemini, OpenAI, Anthropic | Auto-Apply Injection | Analyzes live system stress and generates targeted YAML policies. Includes an interactive prompt to instantly inject the AI-generated payload into the fleet. |

**Supported Providers & Default Models:**
* `--provider openai` (Defaults to `gpt-4o-mini`)
* `--provider gemini` (Defaults to `gemini-2.5-flash`)
* `--provider anthropic` (Defaults to `claude-3-5-sonnet-latest`)

*You can dynamically override the default models using the `-m` or `--model` flag.*

<br>

**Usage Example:**
```bash
# Export your API key
export PASTAAY_AI_KEY="your-api-key"

# Ask Oracle to design a scenario using Gemini (Default model)
pastaayctl oracle "We need to test the database pool limits. Give me a 30s latency config." --provider gemini --health-url [http://api.mycompany.com/health](http://api.mycompany.com/health)

# Override the model to use OpenAI's flagship GPT-4o
pastaayctl oracle "Simulate a cache stampede on Redis" --provider openai -m gpt-4o

```

**Interactive Output Example:**

```text
[#] WAKING PASTAAY ORACLE...
  [*] "We're pushing the boundaries of all that is real and possible. We're not roasting a turkey."
  [*] Scanning fleet topology and active kinetic state...
  [*] Establishing neural link with AI backend...

═══ ORACLE ANALYSIS ═══
Based on your current baseline latency of 45ms and 0 active faults on the SQL layer, here is the optimal blast radius to test connection pooling without triggering a total outage:

```

```yaml
version: 1
policies:
  - name: oracle-db-pool-stress
    type: sql
    target: database
    latency_chance: 0.6
    latency_duration: 30s

```

<br>

*⚠ Note: You can instantly abort this experiment at any time by running `pastaayctl rollback`.*

```text
[?] Oracle has generated a Chaos Policy. Would you like to inject it into the fleet now? (y/N): y
  [*] Discarding safety protocols. Injecting Oracle payload...
[+] PAYLOAD DELIVERED SUCCESSFULLY

```

---

## Operational Reliability Guards

**pastaayctl** is built to be as resilient as the production systems it manipulates:

* **Metric Cardinality Guard:** To protect Prometheus RAM, the CLI respects the engine's 64-character truncation logic for high-entropy metric labels.
* **Memory-Bounded Streams:** All configuration transfers implement `io.LimitReader` (1MB for Webhooks, 5MB for K8s) to mitigate OOM attack vectors against the control plane.
* **Robust Telemetry ACKs:** Commands dispatched via Redis utilize `context.WithoutCancel` to guarantee that "Applied" acknowledgment signals reach the control plane even during unexpected terminal shutdown.

<br>

<p align="center">
  <img src="assets/common_footer.gif" alt="Pastaay Bottom Banner">
</p>
