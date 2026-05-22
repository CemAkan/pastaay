import { API } from './api.js';
import { ChartManager } from './charts.js';
import { UITemplates } from './ui_templates.js';

export class TelemetryPipeline {
    constructor(engine) {
        this.engine = engine;
        this._stateBackoff = 2000;
        this._metricsBackoff = 2000;
        this._stateTimer = null;
        this._metricsTimer = null;
    }

    escapeHTML(value) {
        return String(value ?? '').replace(/[&<>"]/g, (ch) =>
            ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[ch]));
    }

    start() {
        this.engine.pinger.updateHUD();
        this._scheduleState(0);
        this._scheduleMetrics(0);
    }

    _scheduleState(delay) {
        clearTimeout(this._stateTimer);
        this._stateTimer = setTimeout(() => this.fetchState(), delay);
    }
    _scheduleMetrics(delay) {
        clearTimeout(this._metricsTimer);
        this._metricsTimer = setTimeout(() => this.fetchMetrics(), delay);
    }

    fetchState() {
        API.fetchState().then(state => {
            this._stateBackoff = 2000; // success → reset
            this.engine.pinger.updateHUD();
            this._renderState(state);
        }).catch(() => {
            const dot = document.getElementById('engine-status-dot');
            const txt = document.getElementById('engine-status-text');
            if (dot) dot.className = "w-2 h-2 rounded-full bg-rose-500 shadow-[0_0_8px_rgba(244,63,94,0.6)]";
            if (txt) { txt.className = "text-rose-400 font-medium tracking-wide"; txt.innerText = "Offline"; }
            this._stateBackoff = Math.min(this._stateBackoff * 2, 30000);
        }).finally(() => {
            this._scheduleState(this._stateBackoff);
        });
    }

    fetchMetrics() {
        API.fetchMetrics().then(data => {
            this._metricsBackoff = 2000; // success → reset
            if (!data || !Array.isArray(data)) data = [];
            this._renderMetrics(data);
        }).catch(() => {
            this._metricsBackoff = Math.min(this._metricsBackoff * 2, 30000);
        }).finally(() => {
            this._scheduleMetrics(this._metricsBackoff);
        });
    }

    _renderState(state) {
        const dot = document.getElementById('engine-status-dot');
        const txt = document.getElementById('engine-status-text');

        let clusterHealthy = false;
        for (const [name, status] of Object.entries(state.sensors_detail || {})) {
            if (status === 'healthy' || status === 'connected') clusterHealthy = true;
        }

        if (dot && txt) {
            if (clusterHealthy) {
                dot.className = "w-2 h-2 rounded-full bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.4)]";
                txt.className = "text-emerald-500 font-medium tracking-wide"; txt.innerText = "Online";
            } else {
                dot.className = "w-2 h-2 rounded-full bg-amber-500 shadow-[0_0_8px_rgba(245,158,11,0.4)] animate-pulse";
                txt.className = "text-amber-500 font-medium tracking-wide"; txt.innerText = "Degraded (No Sensor)";
            }
        }

        const pc = document.getElementById('policy-count'); if (pc) pc.innerText = state.active_policies;
        const sc = document.getElementById('sensor-count'); if (sc) sc.innerText = state.active_sensors;
        window.currentRawYaml = state.raw_yaml;
        window.activePoliciesCache = state.policies || [];

        if (state.engine_logs && Array.isArray(state.engine_logs)) {
            this.engine.syncBackendLogs(state.engine_logs);
        }

        let sHTML = '';
        for (const [name, status] of Object.entries(state.sensors_detail || {})) {
            let col = (status === 'healthy' || status === 'connected') ? 'text-emerald-500' : 'text-rose-400';
            sHTML += `<li class="flex justify-between items-center py-1.5 border-b border-[#30363d] last:border-0"><span class="text-slate-400">${UITemplates.escapeHTML(name)}</span><span class="${col} font-bold uppercase text-[10px]">${UITemplates.escapeHTML(status)}</span></li>`;
        }
        const sl = document.getElementById('sensors-list'); if (sl) sl.innerHTML = sHTML;

        let tHTML = '';
        if (!state.policies || state.policies.length === 0) {
            tHTML = `<tr><td colspan="4" class="px-5 py-8 text-center text-slate-500 font-mono">System baseline pristine. No active configurations.</td></tr>`;
        } else {
            state.policies.forEach(p => {
                let effect = "";
                let isDrop = p.drop_connection || p.dropConnection || false;
                if (isDrop) effect += `<span class="mr-2 text-purple-400 border border-purple-500/30 bg-purple-950/30 px-2 py-0.5 rounded shadow-sm">Drop</span>`;
                if ((p.error_chance || p.errorChance || 0) > 0 || isDrop) {
                    let code = p.error_code || p.errorCode;
                    if (!code || code === 0) code = isDrop ? "DROP" : "500";
                    effect += `<span class="mr-2 text-rose-400 border border-rose-500/30 bg-rose-950/30 px-2 py-0.5 rounded shadow-sm">Err(${UITemplates.escapeHTML(code)})</span>`;
                }
                let latDur = p.latency_duration || p.latencyDuration;
                let hasLatency = (p.latency_chance || p.latencyChance || 0) > 0;
                if ((hasLatency && latDur) || (p.type === 'resource' && latDur)) {
                    let displayLat = typeof window.formatNsToMs === 'function' ? window.formatNsToMs(latDur) : latDur;
                    effect += `<span class="text-amber-400 border border-amber-500/30 bg-amber-950/30 px-2 py-0.5 rounded shadow-sm">Lat(${UITemplates.escapeHTML(displayLat)})</span>`;
                }
                if (p.type === 'resource' && !latDur) {
                    effect += `<span class="text-slate-400 border border-[#30363d] bg-[#21262d] px-2 py-0.5 rounded shadow-sm">Sabotage</span>`;
                }
                let safeName = UITemplates.escapeHTML(p.name || "unnamed");
                let safeTarget = UITemplates.escapeHTML(p.target || "all");
                let tName = safeName.length > 25 ? safeName.substring(0, 25) + '...' : safeName;
                let tTarget = safeTarget.length > 30 ? safeTarget.substring(0, 30) + '...' : safeTarget;
                tHTML += `<tr class="border-b border-[#30363d]/50 hover:bg-[#21262d]/50 transition-colors"><td class="px-5 py-3.5 font-medium text-slate-200" title="${safeName}">${tName}</td><td class="px-5 py-3.5 text-[10px] text-sky-400 uppercase font-bold tracking-wider">${UITemplates.escapeHTML(p.type)}</td><td class="px-5 py-3.5 text-slate-400" title="${safeTarget}">${tTarget}</td><td class="px-5 py-3.5">${effect}</td></tr>`;
            });
        }
        const pt = document.getElementById('policies-table'); if (pt) pt.innerHTML = tHTML;
    }

    _renderMetrics(data) {
        let now = Date.now();
        let deltaSec = (now - window.lastTime) / 1000.0;
        window.lastTime = now;
        if (deltaSec <= 0 || isNaN(deltaSec)) deltaSec = 2.0;

        let globalRate = 0; let targetMap = {};
        let errorData = {}; let latData = {}; let dropData = {};

        data.forEach(m => {
            let key = m.target + "_" + m.type; let cur = m.value;
            let prev = window.lastMetrics[key] !== undefined ? window.lastMetrics[key] : cur;
            let rate = Math.max(0, (cur - prev) / deltaSec);
            window.lastMetrics[key] = cur; globalRate += rate;

            targetMap[m.target] = (targetMap[m.target] || 0) + cur;
            if (m.type === 'error') errorData[m.target] = cur;
            if (m.type === 'latency') latData[m.target] = cur;
            if (m.type === 'drop') dropData[m.target] = cur;
        });

        this.engine.widgets.list.forEach((w) => {
            if (w.metric === 'global_rate' && w.chart) {
                w.history.shift(); w.history.push(globalRate);
                ChartManager.updateGlobalRate(w.chart, w.history);
                w.chart.resize();

                const list = document.getElementById(`${w.id}-list`);
                if (list) {
                    const peak = Math.max(...w.history);
                    const prevRate = w.history[w.history.length - 2] || 0;
                    const delta = globalRate - prevRate;
                    const deltaColor = delta > 0 ? 'text-rose-400' : 'text-emerald-400';
                    const deltaSign = delta > 0 ? '+' : '';
                    list.innerHTML = UITemplates.buildGlobalRateDetails(globalRate, peak, delta, deltaColor, deltaSign);
                }
            }
            else if (w.metric === 'impact_matrix' && w.chart) {
                let targets = Object.keys(targetMap).sort((a, b) => targetMap[b] - targetMap[a]).slice(0, 5);
                if (targets.length === 0 && window.activePoliciesCache && window.activePoliciesCache.length > 0) {
                    targets = window.activePoliciesCache.map(p => {
                        let tag = p.type + ":" + p.target;
                        return tag.length > 64 ? tag.substring(0, 61) + "..." : tag;
                    }).slice(0, 5);
                }
                ChartManager.updateImpactMatrix(w.chart, targets, errorData, latData, dropData);
                w.chart.resize();

                const list = document.getElementById(`${w.id}-list`);
                if (list) {
                    if (targets.length === 0 || Object.keys(targetMap).length === 0) {
                        list.innerHTML = `<li class="text-slate-500 text-xs italic font-mono p-2">Awaiting system fault data streams...</li>`;
                    } else {
                        let topTarget = targets[0];
                        let totalErrs = targets.reduce((sum, t) => sum + (errorData[t] || 0), 0);
                        let totalLats = targets.reduce((sum, t) => sum + (latData[t] || 0), 0);
                        let totalDrops = targets.reduce((sum, t) => sum + (dropData[t] || 0), 0);
                        list.innerHTML = UITemplates.buildImpactMatrixDetails(topTarget, totalErrs, totalLats, totalDrops);
                    }
                }
            }
        });
    }
}