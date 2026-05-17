<p align="center">
  <img src="docs/assets/contributing_header.png" alt="Contributing Header">
</p>

<br>

First off, thank you for considering contributing to Pastaay! It's people like you that make open-source software such a great community to build, learn, and create.

Pastaay is a highly concurrent, low-latency chaos engineering engine. Because it intercepts application traffic at the driver and network levels, contributions require a careful approach to memory management, thread safety, and interface design. 

This document outlines the process and guidelines for contributing.

---

## Table of Contents
1. [Code of Conduct](#code-of-conduct)
2. [Getting Started](#getting-started)
3. [Development Guidelines & Architecture](#development-guidelines--architecture)
4. [Testing](#testing)
5. [Commit Conventions](#commit-conventions)
6. [Pull Request Process](#pull-request-process)

---

## Code of Conduct
By participating in this project, you are expected to uphold standard open-source community guidelines. Be respectful, constructive, and welcoming to others.

## Getting Started

1. **Fork the repository** on GitHub.
2. **Clone your fork** locally:
   ```bash
   git clone https://github.com/YOUR-USERNAME/pastaay.git
   cd pastaay
   ```
3. **Install dependencies:**
   ```bash
   go mod download
   ```
4. **Create a new branch** for your feature or bugfix:
   ```bash
   git checkout -b feature/my-new-interceptor
   ```

---

## Development Guidelines & Architecture

Pastaay is designed to have **zero blocking overhead** when chaos is not actively triggered. To maintain this, please adhere to the following architectural rules:

### 1. The `config.Manager` is Sacred
All configurations are read from a pre-computed map[string][]Policy accessed via lock-free atomic.Pointer[T].
* **Never** perform I/O operations or file reads inside an interceptor.
* Always use `mgr.GetActivePolicies("your-protocol")` to retrieve policies in `O(1)` time.

### 2. Strict Case-Insensitivity
When matching user targets (e.g., matching a Redis `GET` command against a YAML `target: "get"`), **always** use `strings.EqualFold()`. Never assume the user's YAML input or the driver's output will have a specific case.

### 3. Beware of Driver Fallbacks (SQL & Hooks)
When implementing wrappers (like standard library `database/sql` drivers), be aware of interface fallbacks. For example, if you implement both `Query` and `QueryContext`, ensure that the chaos injection logic doesn't execute twice if the Go compiler falls back from one to the other.

### 4. Pointer Safety in Pipelines
When dealing with batch operations or pipelines (e.g., Redis `ProcessPipelineHook`), do not mutate slice elements via value-copy `range` loops. Always use index-based slice mutations (`cmds[i].SetErr(redis.Nil)`) to ensure the original pointers receive the synthetic errors.

### 5. The Zero-Allocation Mandate
Pastaay's interceptors reside in the ultra-hot critical paths of the host application. You must ensure **zero memory allocation** (0 bytes) during normal evaluation (when chaos is not injected).
* Do not use `make()`, `new()`, or allocate new structs inside the `Intercept` or `Evaluate` methods.
* Avoid string concatenations or byte-slice copies; pass references and use standard library zero-allocation methods where possible.
---

## Testing

Every new feature or bugfix must be accompanied by tests.

1. **Run existing tests** before writing new code:
   ```bash
   go test ./... -v
   ```
2. **Write unit tests** for your new code. If you are adding a new protocol interceptor, include mock tests verifying both latency and error injections.
3. **Verify Demo Integration:** If your change affects the core engine, ensure the Docker Compose demo still boots successfully without crash-loops.

## Commit Conventions

We follow [Conventional Commits](https://www.conventionalcommits.org/). This helps us automatically generate changelogs.

* `feat:` A new feature (e.g., `feat(kafka): add message dropping interceptor`)
* `fix:` A bug fix (e.g., `fix(core): resolve fsnotify file amnesia`)
* `docs:` Documentation only changes
* `test:` Adding missing tests or correcting existing ones
* `chore:` Changes to the build process or auxiliary tools

---

## Pull Request Process

1. Ensure your code follows standard Go formatting
   ```bash
   go fmt ./...
   go vet ./...
    ```
2. Update the `README.md` or `docs/` with details of changes to the interface, new configurations, or new features.
3. Submit a Pull Request targeting the `main` branch (or the current active development branch).
4. Provide a clear and descriptive PR title and fill out the PR description with the *Why* and *How* of your changes.
5. A maintainer will review your code. You may be asked to make structural changes. Once approved, it will be merged!

<br>

<p align="center">
  <img src="docs/assets/contributing_bottom.gif" alt="Contributing Bottom">
</p>