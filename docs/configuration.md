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

###  Deep Dive: Customizing `ignored_commands` & Anti-Bypass

The `ignored_commands` field allows you to explicitly protect certain queries or commands from chaos injection. 

**Crucial Matching Rules:**
1. **Case-Insensitive:** Pastaay automatically converts both your YAML rules and the incoming queries to `UPPERCASE` before comparing. `select` matches `SELECT` and `sElEcT`.
2. **Prefix Matching:** Pastaay uses a prefix matching engine. If you ignore `"CREATE"`, it protects `"CREATE TABLE..."`, `"CREATE INDEX..."`, etc.
3. **Advanced Anti-Bypass Scrubbing (v1.5.1+):** Pastaay strictly scrubs mixed SQL comments (e.g., `/* comment */`, `-- comment`) and whitespaces before evaluation. A query like `/* Bypass Attempt */ CREATE TABLE` will still be correctly identified and protected.

**Complete Map Example:**
```yaml
enable_default_ignored: true
ignored_commands:
  sql:
    - "SELECT 1"      # Protects health check pings
    - "EXPLAIN"       # Protects query planners
  mongo:
    - "ping"          
    - "buildInfo"
  redis:
    - "PING"
    - "AUTH"          
```

---

##  Policy Structure

Each policy in the `policies` list supports the following fields to precisely target and execute chaos:

| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `name` | `string` | No | A unique identifier for the policy (useful for debugging). |
| `type` | `string` | **Yes** | Supported values: `http`, `sql`, `grpc`, `redis`, `mongo`. |
| `target` | `string` | **Yes** | Endpoint, route, or specific query to target. **Strictly Case-Insensitive.** |
| `latency_chance` | `float` | No | Probability (0.0 to 1.0) of injecting latency. |
| `latency_duration`| `string` | No | Delay duration (e.g., `500ms`, `2s`). |
| `error_chance` | `float` | No | Probability (0.0 to 1.0) of injecting a synthetic error/fault. |
| `error_code` | `int` | No | Custom HTTP/gRPC Status Code *(type: http/grpc only)*. |
| `error_body` | `string` | No | Custom response/error message. |
| `drop_connection`| `bool` | No | Forcefully rejects the TCP dial connection. **Safeguard: Requires `target: "all"` or `"database"` to execute.** |
| `match_headers` | `map` | No | Key-value pairs for Blast Radius Control. |

<br>

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
* **Target Format:** Full Method Name (e.g., `/service.v1.MyService/MyMethod`). Evaluated Case-Insensitive.
* **Fault Behavior:** Aborts the request returning a `codes.Unavailable` status.

### 5. Redis Chaos (`type: "redis"`)
* **Target Format:** Specific command (e.g., `"get"`, `"set"`), or `"all"`.
* **Fault Behavior:** Simulates a **Cache Miss** by returning a `redis.Nil` error. Respects pipeline sequence (latency strictly applied *before* physical execution).

---

## Cascading Rules
Pastaay supports cascading chaos rules. A single route can be targeted by multiple policies. For instance, a request can be hit by a latency policy first, and subsequently hit by an error policy without short-circuiting.

## Hot-Reloading Behavior
Pastaay watches this file natively. If you modify `pastaay.yaml` while your application is running, the new policies are loaded instantly. Pastaay is immune to Linux `Vim/Nano` amnesia (file replace events) and will auto-recover the file watcher dynamically.

