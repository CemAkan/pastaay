<p align="center">
  <img src="assets/conf_header.png" alt="Configuration Header">
</p>

The `pastaay.yaml` file is the heart of the chaos engine. The configuration supports global protection rules, granular targeting, and case-insensitive policy matching.

## Global Settings

These settings govern the overall behavior of the Pastaay engine and protect your application during critical startup phases and standard operations.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `version` | `int` | **Yes** | Configuration schema version (currently `1`). |
| `warmup_duration` | `string` | No | Time to wait after boot before injecting any chaos (e.g., `"10s"`). Prevents crash-loops during container initialization. |
| `enable_default_ignored` | `bool` | No | Automatically bypasses chaos for critical setup DDLs (e.g., `CREATE`, `DROP`, `createIndexes`). |
| `ignored_commands` | `map` | No | Custom map of protocols and commands to explicitly protect from fault injection. |

---

## Engine Integrity Guard

Pastaay operates on the principle of **Unrestricted Chaos**. The engine assumes the operator is a domain expert and does not enforce arbitrary "babysitting" limits on latency durations or memory exhaustion. If an operator configures a 5-hour network partition or a 256GB RAM leak, the engine will execute it unconditionally.

However, to prevent the chaos engine *itself* from crashing due to malformed payloads or causing collateral infrastructure damage, it enforces strict structural validation before any memory swap occurs:

* **Memory-Bounded Streams (Network Layer):** Webhook and Kubernetes sensors implement `io.LimitReader` bounds (1MB for webhooks, 5MB for ConfigMaps) to prevent OS-level memory exhaustion (OOM) during JSON/YAML deserialization.
* **Metric Cardinality Protection:** To prevent "Label Explosion" and subsequent Prometheus memory exhaustion, all generated `MetricTag` values are strictly truncated to a maximum of 64 characters. This ensures telemetry stability even if users input raw, high-entropy SQL queries as chaos targets.
* **Logical Sanity:** Rejects negative probabilities, negative durations, and negative resource allocations.
* **Protocol Sanity:**
* **HTTP:** Error codes must be within the valid `100-599` range.
* **gRPC:** Status codes must follow the official Google RPC specification (`0-16`).
* **SQL:** Targets are pre-compiled as regex; invalid regex patterns are rejected instantly to prevent runtime panics.


* **Atomic Rollback:** If any single policy within a batch payload fails structural validation, the entire payload is rejected using `errors.Join`, maintaining the engine's last-known-good state.

---

## Deep Dive: Customizing `ignored_commands` & Anti-Bypass

### Crucial Matching Rules:

* **Case-Insensitivity**: All targets and incoming commands are normalized to `UPPERCASE` before evaluation.
* **SQL Stripping**: The engine strips all standard SQL delimiters, including parentheses `()` and semicolons `;`. A query like `(SELECT 1);` securely matches `SELECT 1` in your policies.
* **Slash Normalization**: Leading slashes in HTTP/gRPC paths are normalized at the edge. `///api/ping` will match `api/ping` precisely, preventing evasion via malformed routing.

### Complete Map Example:

```yaml
enable_default_ignored: true
ignored_commands:
  sql:
    - "SELECT 1"      # Protects database health check pings
    - "EXPLAIN"       # Protects query planners
  mongo:
    - "ping"
    - "buildInfo"
  redis:
    - "PING"
    - "AUTH"
  kafka:
    - "heartbeat"     # Protects consumer group stability

```

---

## Policy Structure

Each policy in the `policies` list supports the following fields to precisely target and execute chaos vectors:

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `name` | `string` | No | A unique identifier for the policy (highly recommended for telemetry tracking). |
| `type` | `string` | **Yes** | Supported values: `http`, `sql`, `grpc`, `redis`, `mongo`, `kafka`, `rabbitmq`, `resource`. |
| `target` | `string` | **Yes** | Endpoint, route, or specific query to target. **Strictly Case-Insensitive.** |
| `latency_chance` | `float` | No | Probability (`0.0` to `1.0`) of injecting a latency delay. |
| `latency_duration` | `string` | No | **Protocols:** Request delay duration (e.g., `"500ms"`). <br> **Resource:** Total attack duration before Amnesia cleanup (e.g., `"15s"`). |
| `error_chance` | `float` | No | Probability (`0.0` to `1.0`) of injecting a synthetic error/fault. |
| `error_code` | `int` | No | Custom HTTP/gRPC Status Code *(type: http/grpc only)*. |
| `error_body` | `string` | No | Custom response payload or error message string. |
| `drop_connection` | `bool` | No | Forcefully rejects the TCP dial connection. **Safeguard: Requires `target: "all"` or `"database"` to execute.** |
| `match_headers` | `map` | No | Key-value pairs for Blast Radius Control (e.g., targeting specific user agents or tokens). |
| `throttle_threshold` | `int` | No | CPU intensity (SHA-256 hashes per context loop). Default: `100,000`. |
| `ram_chunk_mb` | `int` | No | Physical RAM allocation size per interval (MB). |
| `ram_interval` | `string` | No | Frequency of memory page allocation (e.g., `"1s"`). |
| `stream_roll_mode` | `string` | No | gRPC only. Dictates RNG evaluation frequency (`stream` or `message`). Default: `stream`. |

---

### Fault Diagnosis (Validation Rules)

Pastaay maintains absolute stability by enforcing structural rules upon payload ingestion. If a policy violates these constraints, it is rejected entirely.

| Logic | Rule | Reason for Rejection |
| --- | --- | --- |
| **Probability** | `0.0 <= chance <= 1.0` | Strict adherence to probability theory constraints. |
| **Duration** | `duration >= 0` | Negative time deltas induce system clock instability and goroutine deadlocks. |
| **Resources** | `mb >= 0`, `intensity >= 0` | Negative resource allocation is logically impossible and triggers memory panics. |
| **HTTP Spec** | `100 <= code <= 599` | Codes outside this range break standard HTTP clients and ingress controllers. |
| **gRPC Spec** | `0 <= code <= 16` | Codes must follow the official Google gRPC status specification. |
| **SQL Regex** | Must be valid Regex | Invalid regex strings prevent query matching and fail compilation. |

> **Note:** The engine follows the **Unrestricted Chaos** philosophy. There are no upper limits on duration or memory thresholds. If configured to execute extreme values, the engine will proceed. Ensure payloads are planned accordingly.

---

## Observability & Labels

Every fault injected by Pastaay is reported to Prometheus. For granular filtering in Grafana, use the `protocol:target` format.

> **SRE Guardrail:** To protect your metrics backend from high-cardinality label explosions (e.g., targeting a raw SQL query containing hundreds of unique UUIDs), Pastaay strictly truncates the combined `protocol:target` label string to a maximum of **64 characters**.

| Protocol | Label Example | Target Logic |
| --- | --- | --- |
| **HTTP** | `http:/api/v1/login` | Normalized URI path. |
| **SQL** | `sql:database` | Global identifier or query-specific regex match. |
| **gRPC** | `grpc:/pb.Svc/Method` | Full gRPC method name. |
| **Kafka** | `kafka:topic_name` | Exact Kafka topic name. |
| **RabbitMQ** | `rabbitmq:routing_key` | Exact routing key or queue name. |
| **Redis** | `redis:get` | Exact command name or `all`. |

### Remote Control Health Metrics

| Metric | Type | Labels | Description |
| --- | --- | --- | --- |
| `pastaay_remote_sensor_status` | Gauge | `sensor` | `1.0`: Healthy/Connected. `0.0`: Disconnected or Invalid Payload parsing error. |

---

## Target Types & Granular Fault Behavior

How Pastaay interprets faults depends entirely on the `type` declaration of the policy.

### 1. HTTP Chaos (`type: "http"`)

* **Target Format:** URL Path (e.g., `/api/v1/users` or `/api/*` for wildcards).
* **Fault Behavior:** Uses `error_code` for HTTP Status. Uses `error_body` for JSON response. Defaults to `500 Internal Server Error` if omitted.

### 2. SQL Chaos (`type: "sql"`)

* **Target Format:** `"database"`, `"all"`, or Granular Queries (e.g., `"INSERT INTO users"`).
* **Fault Behavior:** Simulates native `database/sql` driver errors or execution delays. *Note: `drop_connection` will only trigger if the target is explicitly global (`"all"` or `"database"`) to prevent accidental localized connection pool exhaustion.*

### 3. MongoDB Chaos (`type: "mongo"`)

* **Target Format:** Specific command (e.g., `"insert"`, `"find"`) or `"all"`.
* **Fault Behavior:** Injects latency at the native BSON event monitor level prior to wire transmission.

### 4. gRPC Chaos (`type: "grpc"`)

Unlike request-response protocols, long-lived gRPC streams require deterministic consistency to avoid breaking application-level state machines.

The `stream_roll_mode` configuration dictates when the Chaos Engine "rolls the dice":

* **`stream` (Default):** The lock-free RNG is evaluated exactly *once* at the initiation of the stream. The decision is cached. This is the safest way to simulate total link failure without triggering illegal state transitions in the Go gRPC runtime.
* **`message`:** The RNG is evaluated independently for *every* `SendMsg` or `RecvMsg` call. Ideal for simulating sporadic network jitter or intermittent packet loss.

**FNV-1a Fingerprinting:**
Pastaay utilizes an FNV-1a hashing algorithm to generate a `PolicyHash`. Even during a `hot-reload`, the engine uses this hash to ensure running streams maintain their original "Chaos Fate" consistently, preventing active streams from flickering between stable and chaotic states.

### 5. Redis Chaos (`type: "redis"`)

* **Target Format:** Specific command (e.g., `"get"`, `"set"`), or `"all"`.
* **Fault Behavior:** Simulates a **Cache Miss** by returning a native `redis.Nil` error. Respects pipeline sequences (latency is strictly applied *before* physical execution of the batch).

### 6. Kafka Chaos (`type: "kafka"`)

* **Target Format:** Kafka Topic Name (e.g., `"user_events"`, `"orders"`), or `"all"`.
* **Fault Behavior:** `error_chance` forces a synthetic, unrecoverable message processing error. `latency_chance` halts the consumer goroutine securely using context-aware `select` blocks to avoid blocking graceful application shutdowns. `drop_connection` explicitly drops the message, bypassing host application logic entirely.

### 7. RabbitMQ Chaos (`type: "rabbitmq"`)

* **Target Format:** Routing Key, Exchange, or Queue Name (e.g., `"payment.processed"`), or `"all"`.
* **Fault Behavior:** Mirrors Kafka behavior. Uses zero-allocation header extraction and enforces strict type-assertion on AMQP headers to guarantee the Chaos Engine never triggers a Go runtime panic during header evaluation.

### 8. Resource Chaos (`type: "resource"`)

Direct host environment manipulation with guaranteed cleanup routines:

* **Target Format:** Use `"host"` or `"system"`.
* **Attack Timer:** Controlled by `latency_duration`.
* **Zero-Footprint:** The **Amnesia Protocol** triggers `runtime.GC()` to forcefully reclaim all leaked RAM instantly after the duration expires, returning the node to a stable state.

---

## Cascading Rules

Pastaay supports cascading chaos rules. A single route can be targeted by multiple policies concurrently. For instance, a request can be intercepted by a latency policy first, and subsequently hit by a distinct error policy without short-circuiting the evaluation chain.

---

## Hot-Reloading: Amnesia-Proof Daemon

Pastaay is heavily hardened against the "Linux File-Save Amnesia" bug. Standard editors (Vim, Nano) and deployment pipelines often delete the original file inode during a save operation, which breaks standard filesystem watchers.

* **Recovery**: Pastaay natively traps `Rename/Remove` filesystem events.
* **Re-attachment**: It engages an asynchronous retry loop to re-attach the `fsnotify` watcher to the new file inode instantly, 

---

## Kubernetes Native Configuration (CRD)

If you are using the **Pastaay Kubernetes Operator**, you do not need to manually distribute `pastaay.yaml` files. Instead, you can define your chaos vectors using the native `ChaosPolicy` Custom Resource Definition (CRD).

The Operator automatically translates these CRDs into standard Pastaay JSON payloads and injects them into the Engine's webhook.

### ChaosPolicy Example

```yaml
apiVersion: chaos.pastaay.io/v1
kind: ChaosPolicy
metadata:
  name: cache-stampede-simulation
  namespace: default
spec:
  type: redis
  target: get
  latencyChance: 0.8
  latencyDuration: "2s"
  errorChance: 0.1
```

### Spec Field Mapping
The CRD `spec` fields map 1:1 with the standard `pastaay.yaml` properties, but they utilize Kubernetes-standard `camelCase` naming conventions instead of `snake_case` (e.g., `latency_chance` becomes `latencyChance`).

**Operator-Specific Fields:**
* `duration`: *(String, Optional)* Exclusive to the Kubernetes Operator. Defines the total time the chaos experiment should run (e.g., `"45s"`, `"2m"`). Once this duration expires, the Operator autonomously triggers a rollback, reverting the cluster to a stable state. Acts as a native Dead Man's Switch.