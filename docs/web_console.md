<p align="center">
  <img src="assets/webconsole_header.gif" alt="Web Console Header">
</p>

# The Web Console 

Pastaay features a built-in, air-gapped Web Console designed for real-time kinetic observability and rapid policy orchestration without touching the terminal.

The console is served directly from the Pastaay Engine's memory via port `:2112/console`, meaning it requires **zero external dependencies** (no Node.js, no React) and works perfectly in highly-secure, air-gapped banking/enterprise environments.

---

## 1. Kinetic Dashboard (Real-Time Telemetry)
The dashboard provides a live X-Ray of your fleet's current state.

* **HTMX Polling:** The system status, connected sensors, and active policy tables are hydrated via HTMX. The UI fetches raw HTML directly from the engine every 2 seconds, ensuring zero flickering and ultra-low CPU overhead.
* **ECharts Integration:** The **Kinetic Impact** bar chart reads the `pastaay_injected_faults_total` Prometheus metric directly from the engine's internal `DefaultGatherer`. This visualizes the exact blast radius and hit counts of your chaos policies in real-time, bridging the gap before data reaches Grafana.

---

## 2. Visual Configurator (Drag & Drop Builder)
Writing YAML manually during an active incident response is error-prone. The **Visual Builder** (`/console/builder`) acts as an interactive GUI for the underlying Chaos Engine.

### How it Works (The Webhook Link)
1. **Design:** You design your chaos experiments using the UI dropdowns (selecting Protocols, Targets, and Error Chances).
2. **Live Preview:** The builder auto-generates Pastaay V1 compliant YAML in real-time using `js-yaml`.
3. **Hot-Swap Deployment:** When you click **Deploy to Engine**, the UI takes the generated YAML, securely attaches your `Webhook Token`, and dispatches an asynchronous `POST` request to the Engine's authenticated `/chaos/webhook` endpoint.
4. **Validation & Security Guard:** The engine strictly validates the incoming payload. The Webhook Token is verified using a `ConstantTimeCompare` algorithm to prevent timing attacks. If successful, it performs an atomic memory hot-swap. If invalid, the UI rejects it gracefully without crashing the engine.

---

## 3. Air-Gapped Documentation
All documentation (including this file and its associated assets) is strictly embedded into the Go binary during compilation using `embed.FS`. The Web Console uses a lightweight Markdown parser (`marked.js`) and `mermaid.js` to render architecture diagrams and reference manuals directly from memory, ensuring docs are always version-locked with your specific engine release.

<br>

<p align="center">
  <img src="assets/common_footer.gif" alt="Web Console Footer">
</p>