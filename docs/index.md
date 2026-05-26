<p align="center">
  <img src="assets/milkshake4drBishop.gif" alt="Pastaay Engine">
</p>

# Welcome to Pastaay Engine

Pastaay is an enterprise-grade chaos engineering platform designed with an absolute zero-allocation philosophy. It operates directly at the driver and network layers to inject faults, simulate degradation, and test the resilience of your distributed systems in real-time.

Instead of relying on heavy proxies or sidecars, Pastaay embeds deeply into your application's critical path and actively streams telemetry straight from your Kubernetes pods into the Air-Gapped Web Console.

### Core Philosophies

* **Zero-Allocation Interceptors:** Built for high-throughput environments. If chaos isn't actively triggered, the engine bypasses evaluation instantly without waking the garbage collector.
* **Unrestricted Chaos:** We don't babysit. If you configure a 5-hour database partition or a 256GB RAM leak, the engine executes it. You are in full control of the blast radius.
* **Amnesia-Proof Synchronization:** Configurations can be swapped in milliseconds via Webhooks, Redis PubSub, or K8s ConfigMaps.
* **Autonomous Oracle AI:** Deep neural connectivity for synthesizing structural failure YAMLs by analyzing live network conditions directly from the cluster.

<br>

👈 **Navigate through the sidebar** to explore the architecture, configure your first payload, or master the remote control capabilities.

<br>

<p align="center">
  <img src="assets/main_footer.png" alt="QR Code">
</p>