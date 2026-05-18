<p align="center">
  <img src="docs/assets/sec_header.gif" alt="Security Header">
</p>

## Supported Versions

Active support and security patches are provided strictly for the current stable major release.

| Version | Supported |
| ------- | --------- |
| v2.2.x  | Yes       |
| < v2.2  | No        |

## Pastaay Security Architecture (Guards)

Pastaay is designed to operate within highly sensitive production microservices. To protect the host application from external exploitation, the engine natively enforces the following security boundaries:

* **Constant-Time Verification:** Webhook authentication tokens are validated using `crypto/subtle.ConstantTimeCompare` to mitigate side-channel timing attacks.
* **OOM Exhaustion Protection:** Network sensors enforce strict memory bounds using `io.LimitReader` (capped at 1MB for webhooks and 5MB for Kubernetes ConfigMaps) to prevent memory allocation exhaustion vectors.
* **Principle of Least Privilege:** The Kubernetes Operator runs inside an isolated namespace (`operator-system`) and utilizes fine-grained RBAC boundaries restricted solely to `ChaosPolicy` custom resources. It never requests cluster-admin or broad workload execution privileges.

## Reporting a Vulnerability

**Do not open public GitHub issues for security vulnerabilities.**

If a security vulnerability is discovered within the Pastaay Chaos Engine or Kubernetes Operator, please report it privately by emailing the maintainer directly at **mail@akancem.com**.

Please include:
1. A detailed description of the vulnerability and the potential exploit vector.
2. Step-by-step reproduction steps or a minimal Proof of Concept (PoC) manifest.
3. The impact on the host microservice or the cluster control plane.

Reports will be acknowledged within 48 hours, followed by a coordinated disclosure timeframe once the patch is prepared and merged into the main release line.

<br>

<p align="center">
  <img src="docs/assets/common_footer.gif" alt="Pastaay Bottom Banner">
</p>