<p align="center">
  <img src="../assets/conf_header.png" alt="Pastaay Logo">
</p>

The `pastaay.yaml` file is the heart of the chaos engine. It uses a policy-based structure, allowing you to define multiple chaos rules that run concurrently.

## Policy Structure

Each policy in the `policies` list supports the following fields:

| Field | Type | Required | Description                                                               |
| :--- | :--- | :--- |:--------------------------------------------------------------------------|
| `name` | `string` | No | A unique identifier for the policy (useful for logging and debugging).    |
| `type` | `string` | **Yes** | The type of chaos target. Supported values: `http`, `sql`, `grpc`, `redis`. |
| `target` | `string` | **Yes** | The specific endpoint, database route, or gRPC method to target.          |
| `latency_chance` | `float` | No | Probability (0.0 to 1.0) of injecting latency.                            |
| `latency_duration`| `string` | No | How long to delay the request (e.g., `500ms`, `2s`, `1.5s`).              |
| `error_chance` | `float` | No | Probability (0.0 to 1.0) of injecting a synthetic error/fault.            |
| `error_code` | `int` | No | Custom HTTP Status Code. *(Only applies to `type: http`)*. Defaults to `500`. |
| `error_body` | `string` | No | Custom response/error message. Behavior varies by `type` (See below). |
| `match_headers` | `map` | No | Key-value pairs for Blast Radius Control.                                 |

---

## Target Types & Fault Behavior

How Pastaay interprets `error_chance`, `error_code`, and `error_body` depends entirely on the `type` of the policy.

### 1. HTTP Chaos (`type: "http"`)
Intercepts standard REST/HTTP requests before they reach your handlers.
* **Target Format:** URL Path (e.g., `/api/v1/users` or `/api/*` for wildcards).
* **Fault Behavior:** Uses `error_code` to set the HTTP Status Code (e.g., 403, 429). If omitted, defaults to 500.
    * Uses `error_body` to write a custom JSON response. If omitted, returns a generic Pastaay error JSON.
### 2. SQL Chaos (`type: "sql"`)
Intercepts database queries at the Go `driver` level.
* **Target Format:** Use `"database"` to target all queries. *(Advanced query targeting coming soon).*
* **Fault Behavior:** `error_code` is **ignored**.
    * Uses `error_body` to simulate native database connection errors (e.g., `pq: connection refused` or `deadlock detected`). The wrapper will abort the query and return this string as a native Go `error`.

### 3. gRPC Chaos (`type: "grpc"`)
Intercepts both Unary and Streaming gRPC calls.
* **Target Format:** The Full Method Name (e.g., `/service.v1.MyService/MyMethod`).
* **Fault Behavior:** Aborts the request and returns a `codes.Unavailable` status error. *(Custom gRPC codes via config are planned for future releases).*

### 4. Redis Chaos (`type: "redis"`)
Intercepts `go-redis/v9` commands to test cache resiliency.
* **Target Format:** Specific command (e.g., `"get"`, `"set"`), or `"all"`.
* **Fault Behavior:** Simulates a **Cache Miss** by forcing the Redis client to return a `redis.Nil` error. Highly effective for testing Cache Stampede protections and database fallback logic.

---

## Blast Radius Control (`match_headers`)

To avoid impacting all users in a production or staging environment, you can restrict chaos using the `match_headers` field. The policy will **only** trigger if the incoming request contains all specified headers.

* **For HTTP:** Matches standard HTTP Headers (e.g., `X-Test-User: "true"`).
* **For gRPC:** Matches gRPC incoming Metadata. **Note:** gRPC metadata keys are automatically converted to lowercase by the framework (e.g., use `x-test-user`, not `X-Test-User`).

---

## Complete Example (The Kitchen Sink)

A production-ready example demonstrating flexible, targeted chaos across multiple infrastructure layers:

```yaml
version: 1
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

  # 2. Simulate database connection refused (Hard DB Failure)
  - name: "sql-connection-drop"
    type: "sql"
    target: "database"
    error_chance: 0.1
    error_body: "pq: connection to server failed: Connection refused"

  # 3. Break the gRPC Live Chat stream
  - name: "grpc-chat-stream-fail"
    type: "grpc"
    target: "/chat.v1.ChatService/ConnectStream"
    error_chance: 0.5
      
  # 4. Force Cache Misses on Redis GET commands to test DB load
  - name: "redis-stampede-test"
    type: "redis"
    target: "get"
    error_chance: 0.5
```
---

## Hot-Reloading Behavior

Pastaay watches this file natively. If you modify `pastaay.yaml` while your application is running, the new policies are loaded into memory instantly without dropping active connections or requiring a restart.

---