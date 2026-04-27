<p align="center">
  <img src="../assets/conf_header.png" alt="Pastaay Logo">
</p>

The `pastaay.yaml` file is the heart of the chaos engine. It uses a policy-based structure, allowing you to define multiple chaos rules that run concurrently.

## Policy Structure

Each policy in the `policies` list supports the following fields:

| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `name` | `string` | No | A unique identifier for the policy (useful for logging and debugging). |
| `type` | `string` | **Yes** | The type of chaos target. Supported values: `http`, `sql`, `grpc`. |
| `target` | `string` | **Yes** | The specific endpoint, database route, or gRPC method to target. |
| `latency_chance` | `float` | No | Probability (0.0 to 1.0) of injecting latency. |
| `latency_duration`| `string` | No | How long to delay the request (e.g., `500ms`, `2s`, `1.5s`). |
| `error_chance` | `float` | No | Probability (0.0 to 1.0) of injecting a synthetic error/fault. |
| `match_headers` | `map` | No | Key-value pairs for Blast Radius Control. |

---

## Supported Types & Targets

### 1. HTTP Chaos (`type: "http"`)
Injects latency or `500 Internal Server Error` into standard REST/HTTP endpoints.
* **Target Format:** URL Path (e.g., `/api/v1/users` or `/api/*` for wildcards).

### 2. SQL Chaos (`type: "sql"`)
Intercepts database queries at the driver level to simulate slow database connections.
* **Target Format:** Currently uses `"database"` to target all queries passing through the wrapped driver. *(Note: Advanced table-level targeting will be available in future releases).*

### 3. gRPC Chaos (`type: "grpc"`)
Intercepts both Unary and Streaming gRPC calls to inject latency or `codes.Unavailable` errors.
* **Target Format:** The Full Method Name (e.g., `/service.v1.MyService/MyMethod`).

---

## Blast Radius Control (`match_headers`)

To avoid impacting all users in a production or staging environment, you can use the `match_headers` field. The chaos policy will **only** trigger if the incoming request contains all the specified headers.

* **For HTTP:** Matches standard HTTP Request Headers (e.g., `X-Test-User: "true"`).
* **For gRPC:** Matches gRPC incoming Metadata. **Note:** gRPC metadata keys are automatically converted to lowercase by the gRPC framework (e.g., use `x-test-user`, not `X-Test-User`).

---

## Complete Example (The Kitchen Sink)

Here is a full, production-ready example demonstrating how to run HTTP, SQL, and gRPC chaos concurrently using targeted blast radius control:

```yaml
version: 1
policies:
  # 1. Delay normal users on the login endpoint (20% chance)
  - name: "http-login-slowdown"
    type: "http"
    target: "/api/v1/login"
    latency_chance: 0.2
    latency_duration: "1500ms"
    error_chance: 0.0

  # 2. Hard fail the checkout endpoint, BUT ONLY for testers
  - name: "http-checkout-breakage"
    type: "http"
    target: "/api/v1/checkout"
    latency_chance: 0.0
    error_chance: 1.0
    match_headers:
      x-test-user: "true"

  # 3. Simulate database degradation across the board
  - name: "sql-global-degradation"
    type: "sql"
    target: "database"
    latency_chance: 0.1
    latency_duration: "3s"

  # 4. Break the gRPC Live Chat stream for a specific mobile version
  - name: "grpc-chat-stream-fail"
    type: "grpc"
    target: "/chat.v1.ChatService/ConnectStream"
    latency_chance: 0.0
    error_chance: 0.5
    match_headers:
      x-client-version: "v1.0.4"
```

## Hot-Reloading Behavior

Pastaay watches this file natively. If you modify pastaay.yaml while your application is running, the new policies are loaded into memory instantly without dropping active connections or requiring a restart.