
<p align="center">
  <img src="../assets/conf_header.png" alt="Configuration Header">
</p>

The `pastaay.yaml` file is the heart of the chaos engine. With the introduction of **Smart Mode in v1.5**, the configuration now supports global protection rules alongside dynamic policies.

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

###  Deep Dive: Customizing `ignored_commands`

The `ignored_commands` field allows you to explicitly protect certain queries or commands from chaos injection. It accepts a `map[string][]string` where the key is the protocol (`sql`, `mongo`, `redis`) and the value is a list of command prefixes.

**Crucial Matching Rules:**
1. **Case-Insensitive:** Pastaay automatically converts both your YAML rules and the incoming queries to `UPPERCASE` before comparing. `select` matches `SELECT` and `sElEcT`.
2. **Prefix Matching (Starts With):** Pastaay uses a prefix matching engine (`strings.HasPrefix`). You do not need to write the entire query. If you ignore `"CREATE"`, it will protect `"CREATE TABLE users"`, `"CREATE INDEX..."`, and `"CREATE DATABASE..."`.
3. **Whitespace Trimming:** Leading and trailing spaces in the incoming query are automatically trimmed before evaluation.

**Complete Map Example:**
```yaml
enable_default_ignored: true # Keeps the built-in protections active
ignored_commands:
  sql:
    - "SELECT 1"      # Protects health check pings (e.g., 'SELECT 1 FROM dual')
    - "EXPLAIN"       # Protects query planners
    - "PRAGMA"        # Protects SQLite setup commands
  mongo:
    - "ping"          # Protects MongoDB health checks
    - "buildInfo"
    - "isMaster"
  redis:
    - "PING"
    - "AUTH"          # Never drop authentication handshakes
```
*Note: If `enable_default_ignored` is true, your custom `ignored_commands` are merged with Pastaay's internal default list.*

---

##  Policy Structure

Each policy in the `policies` list supports the following fields to precisely target and execute chaos:

| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `name` | `string` | No | A unique identifier for the policy (useful for debugging). |
| `type` | `string` | **Yes** | Supported values: `http`, `sql`, `grpc`, `redis`, `mongo`. |
| `target` | `string` | **Yes** | Endpoint, database route, or command to target (Use `"all"` for global target). |
| `latency_chance` | `float` | No | Probability (0.0 to 1.0) of injecting latency. |
| `latency_duration`| `string` | No | Delay duration (e.g., `500ms`, `2s`). |
| `error_chance` | `float` | No | Probability (0.0 to 1.0) of injecting a synthetic error/fault. |
| `error_code` | `int` | No | Custom HTTP Status Code *(type: http only)*. |
| `error_body` | `string` | No | Custom response/error message. |
| `drop_connection`| `bool` | No | **(v1.5+)** Forcefully rejects the physical TCP dial connection *(SQL, Redis, Mongo only)*. |
| `match_headers` | `map` | No | Key-value pairs for Blast Radius Control. |

<br>

---

## Target Types & Fault Behavior

How Pastaay interprets faults depends entirely on the `type` of the policy.

### 1. HTTP Chaos (`type: "http"`)
* **Target Format:** URL Path (e.g., `/api/v1/users` or `/api/*` for wildcards).
* **Fault Behavior:** Uses `error_code` for HTTP Status. Uses `error_body` for JSON response. Defaults to `500` if omitted.

### 2. SQL Chaos (`type: "sql"`)
* **Target Format:** `"database"` for general queries.
* **Fault Behavior:** Simulates native DB errors using `error_body` (e.g., `pq: connection refused`) or drops physical connections before they are established if `drop_connection` is true.

### 3. MongoDB Chaos (`type: "mongo"`)
* **Target Format:** Specific command (e.g., `"insert"`, `"find"`) or `"all"`.
* **Fault Behavior:** Injects latency at the event monitor level, or completely drops the physical dialer connection if `drop_connection` is true.

### 4. gRPC Chaos (`type: "grpc"`)
* **Target Format:** Full Method Name (e.g., `/service.v1.MyService/MyMethod`).
* **Fault Behavior:** Aborts the request returning a `codes.Unavailable` status.

### 5. Redis Chaos (`type: "redis"`)
* **Target Format:** Specific command (e.g., `"get"`, `"set"`), or `"all"`.
* **Fault Behavior:** Simulates a **Cache Miss** by returning a `redis.Nil` error, or uses `drop_connection` to sever the network link.

---

## Blast Radius Control (`match_headers`)

To avoid impacting all users in a production or staging environment, you can restrict chaos using the `match_headers` field. The policy will **only** trigger if the incoming request contains all specified headers.

* **For HTTP:** Matches standard HTTP Headers (e.g., `X-Test-User: "true"`).
* **For gRPC:** Matches gRPC incoming Metadata. **Note:** gRPC metadata keys are automatically converted to lowercase by the framework (e.g., use `x-test-user`, not `X-Test-User`).

---

## Complete Example (The v1.5 Kitchen Sink)

A production-ready example demonstrating flexible, targeted chaos across multiple infrastructure layers:
```yaml
version: 1
warmup_duration: "5s"
enable_default_ignored: true
ignored_commands:
  sql:
    - "SELECT 1"

policies:
  # 1. Simulate a Rate Limit (HTTP 429) ONLY for iOS users
  - name: "http-rate-limit-ios"
    type: "http"
    target: "/api/v1/login"
    error_chance: 1.0
    error_code: 429
    error_body: '{"error": "Too Many Requests", "retry_after": 60}'
    match_headers:
      x-client-os: "ios"

  # 2. Hard network drop for MongoDB
  - name: "mongo-network-failure"
    type: "mongo"
    target: "all"
    drop_connection: true

  # 3. Database lock simulation via latency
  - name: "sql-deadlock-simulation"
    type: "sql"
    target: "database"
    latency_chance: 0.1
    latency_duration: "5s"

  # 4. Force Cache Misses on Redis to test DB load (Stampede)
  - name: "redis-stampede-test"
    type: "redis"
    target: "get"
    error_chance: 0.5
```

---

## Hot-Reloading Behavior

Pastaay watches this file natively. If you modify `pastaay.yaml` while your application is running, the new policies are loaded into the cache memory instantly without dropping active connections or requiring a restart.
