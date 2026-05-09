
<p align="center">
  <img src="../assets/conf_header.png" alt="Configuration Header">
</p>

The `pastaay.yaml` file is the heart of the chaos engine. With the introduction of **Smart Mode in v1.5.1**, the configuration supports global protection rules, granular targeting, and case-insensitive policy matching.

## Global Settings

These settings govern the overall behavior of the Pastaay engine and protect your application during critical phases.

| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `version` | `int` | **Yes** | Configuration schema version (currently `1`). |
| `warmup_duration` | `string` | No | Time to wait after boot before injecting any chaos (e.g., `"10s"`). Prevents crashing during container initialization. |
| `enable_default_ignored` | `bool` | No | Automatically bypasses chaos for critical setup DDLs (e.g., `CREATE`, `DROP`, `createIndexes`). |
| `ignored_commands` | `map` | No | Custom map of protocols and commands to explicitly protect. |

<br>

---

## Deep Dive: Customizing `ignored_commands` & Anti-Bypass

### Crucial Matching Rules:

* **Case-Insensitivity**: All targets and incoming commands are normalized to `UPPERCASE` before evaluation.
* **SQL Stripping**: The engine strips all standard SQL delimiters, including parentheses `()` and semicolons `;`. A query like `(SELECT 1)`; matches `SELECT 1` in your policies.
* **Slash Normalization**: Leading slashes in HTTP/gRPC paths are normalized. `///api/ping` will match `api/ping`.

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

##  Policy Structure

Each policy in the `policies` list supports the following fields to precisely target and execute chaos:

| Field                | Type     | Required | Description                                                                                                                                                      |
|:---------------------|:---------| :--- |:---------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `name`               | `string` | No | A unique identifier for the policy (useful for debugging).                                                                                                             |
| `type`               | `string` | **Yes** | Supported values: `http`, `sql`, `grpc`, `redis`, `mongo`,`kafka`,`rabbitmq`.                                                                                     |
| `target`             | `string` | **Yes** | Endpoint, route, or specific query to target. **Strictly Case-Insensitive.**                                                                                      |
| `latency_chance`     | `float`  | No | Probability (0.0 to 1.0) of injecting latency.                                                                                                                         |
| `latency_duration`   | `string` | No | **Protocols:** Request delay duration (e.g., `500ms`). <br> **Resource:** Total attack duration before Amnesia cleanup (e.g., `15s`).                                  |
| `error_chance`       | `float`  | No | Probability (0.0 to 1.0) of injecting a synthetic error/fault.                                                                                                         |
| `error_code`         | `int`    | No | Custom HTTP/gRPC Status Code *(type: http/grpc only)*.                                                                                                                 |
| `error_body`         | `string` | No | Custom response/error message.                                                                                                                                         |
| `drop_connection`    | `bool`   | No | Forcefully rejects the TCP dial connection. **Safeguard: Requires `target: "all"` or `"database"` to execute.**                                                        |
| `match_headers`      | `map`    | No | Key-value pairs for Blast Radius Control.                                                                                                                              |
| `throttle_threshold` | `int`    | No | CPU intensity (hashes per context check). Default: `100,000`.                                                                                                          |
| `ram_chunk_mb`       | `int`    | No | Physical RAM allocation size per interval (MB).                                                                                                                        |
| `ram_interval`       | `string` | No | Frequency of memory allocation (e.g., `"1s"`).                                                                                                                         |
| `stream_roll_mode`   | `string` | No | gRPC only. Dictates RNG evaluation frequency (`stream` or `message`). Default: `stream`.                                                              |

<br>

---

This information should be placed within the `docs/configuration.md` file, specifically under the **"gRPC Chaos"** subsection within the **"Target Types & Granular Fault Behavior"** section. It should also be reflected as a field in the **"Policy Structure"** table.

Here is the professional English translation and technical breakdown for your documentation:

---

### **gRPC Chaos: Deterministic Stream Logic (Dice Logic)**

Long-lived gRPC streams require high consistency to avoid breaking application-level state machines. The `stream_roll_mode` defines when the Chaos Engine "rolls the dice" to determine if a fault should be injected.

*   **`stream` (Default):**
    *   The dice is rolled exactly **once** at the initiation of the stream.
    *   The decision (Latency or Error) is stored in the `decidedPolicies` map.
    *   This result is applied consistently to every message throughout the stream's lifecycle.
    *   **Use Case:** This is the safest way to simulate total link failure or constant degradation without triggering illegal state transitions in the Go gRPC runtime.

*   **`message`:**
    *   The dice is rolled independently for **every** `SendMsg` or `RecvMsg` call.
    *   Decisions are not cached.
    *   **Use Case:** Ideal for simulating intermittent glitches, packet loss, or sporadic jitter in high-frequency data streams.


*   **FNV-1a Fingerprinting:**
    *   Pastaay utilizes an FNV-1a based fingerprinting algorithm to generate a `PolicyHash`.
    *   This hash ensures that the engine recognizes the specific policy identity even during a **hot-reload**.
    *   This allows active streams to maintain their "Chaos Fate" consistently even if the `pastaay.yaml` file is modified and re-parsed while the stream is open.

---

**Note for Implementation:** Ensure you update the **Policy Structure** table to include the `stream_roll_mode` field as an optional string for the `grpc` type.



---

## Observability & Labels

Every fault injected by Pastaay is reported to Prometheus. For granular filtering in Grafana, use the `protocol:target` format:

| Protocol | Label Example | Target logic |
| :--- | :--- | :--- |
| **HTTP** | `http:/api/v1/login` | Normalized path. |
| **SQL** | `sql:database` | Global or query-specific regex match. |
| **gRPC** | `grpc:/pb.Svc/Method` | Full method name. |
| **Kafka** | `kafka:topic_name` | Exact topic name. |
| **RabbitMQ** | `rabbitmq:routing_key`| Exact routing key or queue. |
| **Redis** | `redis:get` | Exact command name or `all`. |

---

## Target Types & Granular Fault Behavior

How Pastaay interprets faults depends entirely on the `type` of the policy.

### 1. HTTP Chaos (`type: "http"`)
* **Target Format:** URL Path (e.g., `/api/v1/users` or `/api/*` for wildcards).
* **Fault Behavior:** Uses `error_code` for HTTP Status. Uses `error_body` for JSON response. Defaults to `500` if omitted.

### 2. SQL Chaos (`type: "sql"`)
* **Target Format:** `"database"`, `"all"`, or Granular Queries (e.g., `"INSERT INTO users"`).
* **Fault Behavior:** Simulates native DB errors or delays. *Note: `drop_connection` will only trigger if the target is global (`"all"` or `"database"`) to prevent accidental localized connection nukes.*

### 3. MongoDB Chaos (`type: "mongo"`)
* **Target Format:** Specific command (e.g., `"insert"`, `"find"`) or `"all"`.
* **Fault Behavior:** Injects latency at the event monitor level.

### 4. gRPC Chaos (`type: "grpc"`)
Unlike request-response protocols, long-lived gRPC streams require deterministic consistency to avoid breaking application-level state machines.

The `stream_roll_mode` configuration dictates when the Chaos Engine "rolls the dice":
* **`stream` (Default):** The lock-free RNG is evaluated exactly *once* at the initiation of the stream. The decision is cached. This is the safest way to simulate total link failure without triggering illegal state transitions in the Go gRPC runtime.
* **`message`:** The RNG is evaluated independently for *every* `SendMsg` or `RecvMsg` call. Ideal for simulating sporadic jitter or intermittent packet loss.

**FNV-1a Fingerprinting:**
Pastaay utilizes an FNV-1a hashing algorithm to generate a `PolicyHash`. Even during a `hot-reload`, the engine uses this hash to ensure running streams maintain their original "Chaos Fate" consistently, preventing active streams from flickering between stable and chaotic states.
### 5. Redis Chaos (`type: "redis"`)
* **Target Format:** Specific command (e.g., `"get"`, `"set"`), or `"all"`.
* **Fault Behavior:** Simulates a **Cache Miss** by returning a `redis.Nil` error. Respects pipeline sequence (latency strictly applied *before* physical execution).

### 6. Kafka Chaos (`type: "kafka"`)
* **Target Format:** Kafka Topic Name (e.g., `"user_events"`, `"orders"`), or `"all"`.
* **Fault Behavior:** `error_chance` forces a synthetic unrecoverable message error. `latency_chance` halts the consumer goroutine securely using context-aware `select` blocks to avoid blocking graceful shutdowns. `drop_connection` explicitly drops the message, bypassing the application logic entirely.

### 7. RabbitMQ Chaos (`type: "rabbitmq"`)
* **Target Format:** Routing Key, Exchange, or Queue Name (e.g., `"payment.processed"`), or `"all"`.
* **Fault Behavior:** Mirrors Kafka behavior. Uses zero-allocation header extraction and enforces strict type-assertion on AMQP headers to guarantee the Chaos Engine never triggers a Go runtime panic.

### 8. Resource Chaos (`type: "resource"`)
Direct host environment manipulation with guaranteed cleanup:
* **Target Format:** Use `"host"` or `"system"`.
* **Attack Timer:** Controlled by `latency_duration`.
* **Zero-Footprint:** The **Amnesia Protocol** triggers `runtime.GC()` to reclaim all leaked RAM instantly after duration expires.

---

## Cascading Rules
Pastaay supports cascading chaos rules. A single route can be targeted by multiple policies. For instance, a request can be hit by a latency policy first, and subsequently hit by an error policy without short-circuiting.

---

## Hot-Reloading: Amnesia-Proof Daemon

Pastaay is hardened against the "Linux File-Save Amnesia" bug. Standard editors like Vim or Nano delete the original file inode during a save.

* **Recovery**: Pastaay natively traps Rename/Remove events.

* **Re-attachment**: It engages an asynchronous retry loop to re-attach the fsnotify watcher to the new file inode instantly, ensuring zero configuration downtime.

---
