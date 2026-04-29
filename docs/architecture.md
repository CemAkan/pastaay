<p align="center">
  <img src="../assets/arch_header.png" alt="Architecture Header">
</p>

Welcome to the engine room of Pastaay. This document explains the core design decisions, memory management, interface wrapping techniques, and the concurrency models that allow the Chaos Engine to inject faults into high-throughput microservices without degrading baseline performance.

---

## 1. The Policy Engine & Memory Management

The biggest challenge in Chaos Engineering is ensuring the tool itself doesn't become a bottleneck. If a service handles 10,000 Requests Per Second (RPS), evaluating chaos rules via a naive `O(N)` loop creates millions of redundant operations.

### The Atomic Map Swap
Pastaay solves this using the `config.Manager`. It acts as an ultra-fast, thread-safe memory cache.
Instead of evaluating the YAML file on every request, the Manager pre-computes a routing map: `map[string][]Policy`.

When an interceptor asks for policies (e.g., `GetActivePolicies("http")`), it performs an `O(1)` map lookup.

### Concurrency Model (`sync.RWMutex`)
To support **Hot-Reloading** without dropping requests, Pastaay uses Go's `sync.RWMutex`:
*   **The Read Path (Interceptors):** `mu.RLock()`. Multiple goroutines (handling API requests) can hold a read lock simultaneously. No request blocks another request.
*   **The Write Path (Hot-Reload):** `mu.Lock()`. When the YAML updates, the Manager waits for active nanosecond-reads to finish, acquires an exclusive lock, swaps the old map pointer with the newly parsed map, and unlocks. This guarantees zero race conditions.

---

## 2. The Interceptor Architecture (How we hook in)

Pastaay does not require you to rewrite your application logic. It uses the **Decorator/Wrapper Pattern** to inject chaos at the lowest possible infrastructure boundaries in Go.

### 2.1 HTTP Chaos (The Decorator Pattern)
Pastaay provides a standard `func(http.Handler) http.Handler` middleware.
It intercepts the `*http.Request` before it reaches your router. If an error policy triggers, it calls `w.WriteHeader(customCode)`, writes the JSON payload, and immediately `return`s, preventing your actual business logic from executing.

### 2.2 SQL Chaos (Driver Interface Wrapping)
This is where Pastaay gets deep into Go's internals. We do not wrap `*sql.DB`. Instead, we implement the `database/sql/driver` interfaces.
1.  We register a custom driver: `sqlchaos.Register("pastaay-postgres", &pq.Driver{}, mgr)`.
2.  Pastaay implements `driver.Driver`, `driver.Conn`, `driver.Queryer`, and `driver.Execer`.
3.  When your app calls `db.Query()`, the standard library passes it to our `WrapperConn`.
4.  We check the Chaos Engine. If a delay is triggered, we call `time.Sleep()`. If an error is triggered, we return a synthetic Go `error` back to the standard library. Otherwise, we pass the query down to the real `pq` (Postgres) driver.

### 2.3 MongoDB Chaos (Event-Driven Monitors)
Unlike SQL, the official Mongo Go Driver (v2) uses an event-based monitoring system.
Pastaay injects a `event.CommandMonitor` into the MongoDB `ClientOptions`.
Before Mongo sends a BSON wire message, it triggers our `Started` hook. Pastaay evaluates the chaos rules here. If network sabotage (`drop_connection`) is enabled, we intercept the `Dialer` inside the `ServerMonitor` and physically sever the TCP connection attempt.

### 2.4 gRPC Chaos (Unary & Stream Interceptors)
Pastaay implements standard `grpc.UnaryServerInterceptor` and `grpc.StreamServerInterceptor`.
Because gRPC uses HTTP/2 under the hood, Pastaay intercepts the RPC call, reads the `FullMethod` name (e.g., `/service.v1.MyService/MyMethod`), and if chaos is triggered, returns a native `status.Error(codes.Unavailable, "chaos injected")`.

---

## 3. The Hot-Reload Daemon (Fault Tolerance)

The ability to update `pastaay.yaml` on the fly is powered by a background daemon. But what happens if a user accidentally saves a corrupted/invalid YAML file?

### The Fail-Safe Mechanism
1.  The file watcher detects a filesystem `Write` event.
2.  Pastaay reads the file and attempts to unmarshal it into a temporary struct.
3.  **Validation:** It runs strict validation (e.g., checking if `error_chance` is between `0.0` and `1.0`).
4.  **Atomic Rollback:** If the YAML is invalid or corrupted, Pastaay logs a severe warning (`[Pastaay] Invalid config, ignoring update...`) and **aborts the update**. The `Manager` continues serving the last-known-good configuration. Your application will never crash due to a typo in the chaos config.

---

## 4. The Observability Pipeline (Prometheus)

Injecting chaos is useless if you cannot measure its impact. Pastaay includes native Prometheus instrumentation, but observability must not add latency.

### Non-Blocking Metrics
Pastaay defines specialized `prometheus.CounterVec` and `prometheus.HistogramVec` metrics.
When a fault is injected, Pastaay increments the counter using `.WithLabelValues(protocol, target, fault_type).Inc()`.
In the Go Prometheus client library, `.Inc()` is implemented using atomic memory operations (`sync/atomic.AddUint64`), which execute at the hardware level in sub-nanoseconds. This means observability is entirely non-blocking and lock-free.