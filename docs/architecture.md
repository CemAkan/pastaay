<p align="center">
  <img src="../assets/arch_header.png" alt="Architecture Header">
</p>

Welcome to the engine room of Pastaay. This document explains the core design decisions, memory management, and the deep OS/Compiler integrations that allow the Chaos Engine to inject faults reliably into high-throughput microservices.

---

## 1. The Policy Engine & Concurrency

The biggest challenge in Chaos Engineering is ensuring the tool itself doesn't become a bottleneck. 

### The Atomic Map Swap
Instead of evaluating the YAML file on every request, the `config.Manager` acts as an ultra-fast, thread-safe memory cache. It pre-computes a routing map: `map[string][]Policy`.
When an interceptor asks for policies, it performs an `O(1)` map lookup.

### Concurrency Model (`sync.RWMutex`)
To support Hot-Reloading without dropping requests, Pastaay uses Go's `sync.RWMutex`:
*   **The Read Path:** `mu.RLock()`. API requests can hold a read lock simultaneously. Zero blocking.
*   **The Write Path:** `mu.Lock()`. During updates, the Manager swaps the old map pointer with the newly parsed map atomically, guaranteeing zero race conditions.

---

## 2. Infrastructure Hooks & Interceptor Architecture

Pastaay uses the **Decorator/Wrapper Pattern** to inject chaos at the lowest possible boundaries.

### 2.1 SQL Chaos: Avoiding The "Double-Chaos" Trap
Pastaay registers a custom driver (`sqlchaos.Register`) and implements the standard `database/sql/driver` interfaces.
**The Fallback Evasion Shield:** The Go `database/sql` library heavily relies on interface fallbacks (e.g., if a driver lacks `ExecContext`, it falls back to `Exec`). Pastaay natively detects these compiler fallbacks at runtime. By selectively suppressing chaos in `Prepare` interfaces and enforcing it directly on `Context` execution interfaces, Pastaay completely eradicates the "Double-Chaos" trap (where a single query gets penalized twice).

### 2.2 Redis Pipelines: Pointer Memory Traps
Injecting errors into a batch of pipelined Redis commands introduces significant slice memory challenges. Modifying a command via a traditional `for _, cmd := range cmds` loop creates a temporary copy, causing the error injection to silently vanish. Pastaay uses strict memory indexing (`cmds[i]`) and enforces a pre-execution chronological delay to guarantee that latency is applied *before* the physical wire message is sent, simulating true network latency.

### 2.3 MongoDB & TCP Nuclear Locks
Unlike SQL, Pastaay injects an `event.CommandMonitor` directly into MongoDB's native Driver (v2). 
For network sabotages (`drop_connection`), Pastaay hijacks the underlying `net.Dialer`. **Safeguard:** Dropping a physical TCP connection is catastrophic. Pastaay enforces a strict "Nuclear Lock", ensuring that TCP Drops can only be executed if the user explicitly targets `"all"` or `"database"`. A localized policy (e.g., targeting a `GET` command) cannot accidentally nuke the connection pool.

---

## 3. Amnesia-Proof Daemon (Fault Tolerance)

Hot-reloading a config file on Linux using standard tools (`Vim`, `Nano`, or `CI/CD`) doesn't simply "write" to a file. It creates a temporary file, deletes the original, and renames the temp file (atomic save). 

Naive `fsnotify` watchers go permanently blind when the original inode is deleted.
**Pastaay's Amnesia-Proofing:** 
1. Pastaay natively traps `Rename/Remove` filesystem events.
2. It engages an asynchronous retry loop to forcibly re-attach to the new file inode.
3. Once attached, it manually forces an unconditional trigger of the `reloadCallback` to prevent the skipped write events from desyncing the policy memory. 

Furthermore, strict YAML validation ensures that corrupted configurations trigger an **Atomic Rollback**, maintaining the last-known-good state to prevent application crashes.

---

## 4. The Observability Pipeline (Prometheus)

Pastaay includes native Prometheus instrumentation (`pastaay_injected_faults_total`).
When a fault is injected, Pastaay increments the counter using atomic memory operations (`sync/atomic.AddUint64`), which execute at the hardware level in sub-nanoseconds. This means observability is entirely non-blocking and lock-free.
---