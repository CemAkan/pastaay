const escapeHTML = (value) => String(value ?? '').replace(/[&<>"]/g, (ch) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[ch]));

export const TelemetryContexts = {
    'pinger': `
        <h4 class="text-sm font-bold text-[var(--text-primary)] mb-2">System Resilience Probe</h4>
        <p class="mb-2">Continuously sends HTTP probes to measure target response time and health.</p>
        <p class="mb-1"><b>Score</b> — EMA-smoothed 0-100 resilience rating. 90+ healthy, 70- degraded, below failing.</p>
        <p class="mb-2">Expand the details panel for per-metric breakdowns and tuning controls.</p>
    `,
    'impact_matrix': `
        <h4 class="text-sm font-bold text-[var(--text-primary)] mb-2">Blast Radius Matrix</h4>
        <p class="mb-2">Stacked horizontal bar chart of the <b>top 5 most-targeted services</b> by fault injection volume. Each bar is segmented into:</p>
        <ul class="list-disc pl-4 space-y-1 mb-2">
            <li><b>Errors (red)</b> — HTTP 5xx and protocol-level errors injected</li>
            <li><b>Latency Spikes (amber)</b> — artificial delay injections</li>
            <li><b>Dropped Connections (purple)</b> — TCP-level disconnects</li>
        </ul>
        <p>Reads directly from the engine's Prometheus gatherer (<code>pastaay_injected_faults_total</code>). The details sidebar shows exact counts aggregated across all displayed targets.</p>
    `,
    'global_rate': `
        <h4 class="text-sm font-bold text-[var(--text-primary)] mb-2">Global Fault Velocity</h4>
        <p class="mb-2">Real-time line chart of the <b>total fault injection rate</b> across all active policies, measured in requests per second.</p>
        <p class="mb-1">Powered by ECharts with a 60-second sliding window. Each data point represents the aggregate faults injected in that tick interval.</p>
        <p>The details sidebar shows <b>Current Velocity</b>, <b>Peak Velocity</b>, and <b>Delta</b> (change from previous tick).</p>
    `,
    'log': `
        <h4 class="text-sm font-bold text-[var(--text-primary)] mb-2">System Output Journal</h4>
        <p class="mb-2">Lock-free circular log viewer streaming pod-level telemetry via the Kubernetes Watch API. Key features:</p>
        <ul class="list-disc pl-4 space-y-1 mb-2">
            <li><b>600-line ring buffer</b> — oldest entries evicted when full</li>
            <li><b>Hierarchical filtering</b> — Pod &rarr; Protocol &rarr; Method drill-down</li>
            <li><b>Click-to-decrypt</b> — structured payload, Trace ID, Span ID, Jaeger deep-link</li>
            <li><b>Live/Pause toggle</b> — freeze the stream for inspection</li>
        </ul>
        <p>Each pod gets a persistent random color for visual distinction. Protocol names are color-coded by type.</p>
    `
};

export const PingerFieldInfo = {
    'pp-apdex': `Satisfied / Tolerating / Frustrated breakdown. Requests below T threshold are <b>Satisfied</b>, up to 4T are <b>Tolerating</b>, above or errors are <b>Frustrated</b>.`,
    'pp-errvel': `Percentage of probes falling into the <b>Frustrated</b> bucket. A sharp rise signals the target is actively failing.`,
    'pp-pct':  `Latency percentiles over the rolling window: <b>P50</b> (median), <b>P95</b>, and <b>P99</b> in milliseconds. Useful for spotting outliers.`,
    'pp-status': `Status signature of the last request. <b>http_200</b> = success, <b>timeout</b> = time-out, <b>network</b> = connection error.`,
    'pp-exc':   `Detailed text of the last error, including target URL, HTTP status code, or full network error description.`
};

// pinger detail field popover HTML
const buildPingerFieldPopover = (fieldId) => `
    <div id="popover-${fieldId}" class="hidden absolute left-0 top-full mt-1 w-[260px] bg-[var(--bg-widget)] border border-[var(--border-color)] shadow-2xl rounded-md p-3 font-sans text-[11px] text-[var(--text-muted)] leading-relaxed break-words whitespace-normal z-[60]">
        ${PingerFieldInfo[fieldId] || '—'}
    </div>`;

window.openMetricInfo = function(metricName, btn, event) {
    if(event) event.stopPropagation();

    ['pinger', 'impact_matrix', 'global_rate', 'log'].forEach(m => {
        const el = document.getElementById(`popover-${m}`);
        if(el && m !== metricName) el.classList.add('hidden');
    });

    const pop = document.getElementById(`popover-${metricName}`);
    if(pop) pop.classList.toggle('hidden');
};

window.togglePingerFieldInfo = function(btn, fieldId, event) {
    if(event) event.stopPropagation();

    const pop = document.getElementById('popover-' + fieldId);
    if (!pop) return;

    // Hide other pinger popovers
    document.querySelectorAll('[id^="popover-pp-"]').forEach(p => {
        if (p.id !== 'popover-' + fieldId) p.classList.add('hidden');
    });

    // Toggle visibility
    pop.classList.toggle('hidden');
};

// Global click-to-close handler
document.addEventListener('click', function(e) {
    if (!e.target.closest('button[onclick^="window.openMetricInfo"]') && !e.target.closest('[id^="popover-"]')) {
        ['pinger', 'impact_matrix', 'global_rate', 'log'].forEach(m => {
            const pop = document.getElementById(`popover-${m}`);
            if(pop) pop.classList.add('hidden');
        });
    }
    if (!e.target.closest('button[onclick^="window.togglePingerFieldInfo"]') && !e.target.closest('[id^="popover-pp-"]')) {
        document.querySelectorAll('[id^="popover-pp-"]').forEach(p => p.classList.add('hidden'));
    }
    if (!e.target.closest('.source-dropdown-container')) {
        document.querySelectorAll('[id$="-source-popover"]').forEach(p => p.classList.add('hidden'));
    }
});

const INFO_ICON_SVG = `<svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>`;

export const UITemplates = {
    escapeHTML: escapeHTML,

    setBtnState: (btn, label, className) => {
        if(!btn) return;
        btn.innerHTML = label;
        if(className) btn.className = className;
    },

    buildGlobalRateDetails: (rate, peak, delta, deltaColor, deltaSign) => {
        return `
            <li class="flex justify-between items-center py-2 border-b border-[var(--border-color)]">
                <span class="text-[var(--text-muted)] text-[11px] uppercase tracking-widest font-bold">Current Velocity</span>
                <span class="font-mono text-xs text-[var(--text-primary)] font-bold">${rate.toFixed(1)} req/s</span>
            </li>
            <li class="flex justify-between items-center py-2 border-b border-[var(--border-color)]">
                <span class="text-[var(--text-muted)] text-[11px] uppercase tracking-widest font-bold">Peak Velocity</span>
                <span class="font-mono text-xs text-[var(--text-primary)] font-bold">${peak.toFixed(1)} req/s</span>
            </li>
            <li class="flex justify-between items-center py-2 border-b border-[var(--border-color)]">
                <span class="text-[var(--text-muted)] text-[11px] uppercase tracking-widest font-bold">Delta (Tick)</span>
                <span class="font-mono text-xs font-bold ${deltaColor}">${deltaSign}${delta.toFixed(1)}</span>
            </li>
        `;
    },

    buildImpactMatrixDetails: (target, errs, lats, drops) => {
        return `
            <li class="flex flex-col py-2 border-b border-[var(--border-color)]">
                <span class="text-[9px] uppercase tracking-widest text-[var(--text-muted)] font-bold mb-1">Primary Target Scope</span>
                <span class="font-mono text-xs text-[var(--accent-cyan)] font-bold truncate" title="${target}">${target}</span>
            </li>
            <li class="flex justify-between items-center py-2 border-b border-[var(--border-color)]">
                <span class="text-[var(--text-muted)] text-[11px] uppercase tracking-widest font-bold">Errors (5xx/Err)</span>
                <span class="font-mono text-xs font-bold text-rose-400">${errs}</span>
            </li>
            <li class="flex justify-between items-center py-2 border-b border-[var(--border-color)]">
                <span class="text-[var(--text-muted)] text-[11px] uppercase tracking-widest font-bold">Latency Spikes</span>
                <span class="font-mono text-xs font-bold text-amber-400">${lats}</span>
            </li>
            <li class="flex justify-between items-center py-2 border-b border-[var(--border-color)]">
                <span class="text-[var(--text-muted)] text-[11px] uppercase tracking-widest font-bold">Dropped (TCP)</span>
                <span class="font-mono text-xs font-bold text-purple-400">${drops}</span>
            </li>
        `;
    },

    updateLintUI: (plan) => {
        const badge = document.getElementById('risk-badge');
        const issuesEl = document.getElementById('lint-issues');
        if(!badge || !issuesEl) return;

        const status = (plan && plan.status) ? String(plan.status).toUpperCase() : 'SAFE';
        const riskMap = {
            SAFE: { color: "text-emerald-400 border-emerald-700/40 bg-[#1f2a1f]", icon: "✔" },
            ELEVATED: { color: "text-sky-400 border-sky-700/40 bg-sky-950/30", icon: "⚠" },
            HIGH: { color: "text-amber-400 border-amber-700/40 bg-[#2a241f]", icon: "⚠" },
            CRITICAL: { color: "text-rose-400 border-rose-700/40 bg-[#2a1f1f]", icon: "☠" }
        };
        const theme = riskMap[status] || riskMap.SAFE;

        badge.innerHTML = `<span class="mr-1">${theme.icon}</span> ${status} RISK`;
        badge.className = `px-3 py-1 rounded text-[10px] font-mono font-bold tracking-widest border shadow-sm transition-colors ${theme.color}`;

        const issues = Array.isArray(plan?.issues) ? plan.issues : [];
        let html = '';

        if (plan && plan.score !== undefined) {
            const scoreColor = plan.score >= 70 ? 'bg-rose-500' : (plan.score >= 30 ? 'bg-amber-500' : 'bg-emerald-500');
            html += `
            <div class="mb-4 border-b border-[#30363d] pb-4">
                <div class="flex justify-between items-center text-[10px] font-mono text-slate-400 mb-1.5">
                    <span class="uppercase tracking-widest font-bold">Blast Radius Entropy</span>
                    <span class="font-bold">${plan.score}%</span>
                </div>
                <div class="w-full bg-[#0d1117] rounded-full h-1.5 border border-[#30363d] overflow-hidden">
                    <div class="${scoreColor} h-1.5 rounded-full transition-all duration-500" style="width: ${plan.score}%"></div>
                </div>
            </div>`;
        }

        if (issues.length > 0) {
            html += `<ul class="space-y-2.5 text-xs font-mono">`;
            html += issues.map(issue => {
                let issueStr = typeof issue === 'string' ? issue : (issue.message || issue.msg || String(issue));
                let label = "INFO", labelColor = "text-sky-400";
                if (issueStr.includes("Conflict") || issueStr.includes("Danger") || issueStr.includes("Scope")) { label = "WARN"; labelColor = "text-amber-400"; }
                if (issueStr.includes("timeout") || issueStr.includes("OOM") || issueStr.includes("immediate")) { label = "FATAL"; labelColor = "text-rose-400"; }

                return `<li class="flex items-start gap-2 bg-[#0d1117] p-2.5 rounded border border-[#30363d]/50 shadow-inner">
                    <span class="${labelColor} font-bold text-[9px] mt-0.5 uppercase tracking-widest">[${label}]</span>
                    <span class="text-slate-300 leading-snug">${escapeHTML(issueStr)}</span>
                </li>`;
            }).join('');
            html += `</ul>`;
        } else {
            html += `<div class="flex items-center justify-center h-full text-slate-500 font-mono text-xs p-4 bg-[#0d1117] rounded border border-[#30363d]/50 border-dashed">No violations detected. System pristine.</div>`;
        }
        issuesEl.innerHTML = html;
    },

    getWrapper: (id, metric, titleStr, expandText, contentHTML) => {
        const btnHtml = `<button id="${id}-expand-btn" onclick="window.engine.toggleWidgetExpand('${id}')" class="px-2 py-1 bg-[#21262d] text-[var(--text-muted)] hover:text-white rounded border border-[#30363d] text-[10px] font-bold uppercase transition-colors focus:outline-none shadow-sm">${expandText}</button>`;
        let detailsHTML = "";

        if (metric === 'log') {
            detailsHTML = `
                <div id="${id}-analysis" class="w-full h-full flex flex-col">
                    <p class="uppercase text-[9px] text-[var(--text-muted)] font-bold border-b border-[var(--border-color)] pb-2 mb-3 tracking-widest flex-shrink-0">Payload Decryption</p>
                    <div id="${id}-analysis-content" class="text-[var(--text-primary)] text-xs font-mono overflow-y-auto flex-1 scrollbar-thin pr-1">
                        Select a log entry to decode structured payload and traces.
                    </div>
                </div>`;
        } else if (metric === 'pinger') {
            detailsHTML = `
                <div class="h-full overflow-y-auto pr-2 pb-4 scrollbar-thin relative">
                    <span class="text-[10px] tracking-widest text-[var(--text-muted)] font-bold border-b border-[var(--border-color)] pb-2 mb-3 block uppercase whitespace-nowrap">Advanced Tuning</span>
                    <div class="grid grid-cols-2 gap-3 mb-4 border-b border-[var(--border-color)] pb-4">
                        <label class="flex flex-col gap-1.5 relative">
                            <span class="text-[9px] tracking-widest text-[var(--text-muted)] font-bold flex items-center gap-1 uppercase whitespace-nowrap">Apdex T (ms)</span>
                            <input type="number" min="50" step="50" value="100" oninput="window.engine.updatePingerSetting('threshold', this.value)" class="bg-[var(--bg-base)] border border-[var(--border-color)] rounded px-2.5 py-1.5 text-xs font-mono text-[var(--text-primary)] outline-none focus:border-[var(--accent-blue)]">
                        </label>
                        <label class="flex flex-col gap-1.5 relative">
                            <span class="text-[9px] tracking-widest text-[var(--text-muted)] font-bold flex items-center gap-1 uppercase whitespace-nowrap">EMA Alpha</span>
                            <input type="number" min="0.05" max="0.5" step="0.05" value="0.2" oninput="window.engine.updatePingerSetting('alpha', this.value)" class="bg-[var(--bg-base)] border border-[var(--border-color)] rounded px-2.5 py-1.5 text-xs font-mono text-[var(--text-primary)] outline-none focus:border-[var(--accent-blue)]">
                        </label>
                        <label class="flex flex-col gap-1.5 relative">
                            <span class="text-[9px] tracking-widest text-[var(--text-muted)] font-bold flex items-center gap-1 uppercase whitespace-nowrap">Interval (ms)</span>
                            <input type="number" min="100" step="100" value="1000" oninput="window.engine.updatePingerSetting('interval', this.value)" class="bg-[var(--bg-base)] border border-[var(--border-color)] rounded px-2.5 py-1.5 text-xs font-mono text-[var(--text-primary)] outline-none focus:border-[var(--accent-blue)]">
                        </label>
                    </div>
                    <span class="text-[10px] tracking-widest text-[var(--text-muted)] font-bold mb-2 block uppercase whitespace-nowrap">Diagnostic Output</span>

                    <ul class="space-y-2 text-xs font-mono text-[var(--text-muted)]" id="pinger-details-list">
                        
                        <li class="flex justify-between items-center py-1.5 border-b border-[var(--border-color)] whitespace-nowrap">
                            <span class="text-[var(--text-muted)] inline-flex items-center gap-1.5 relative">
                                Apdex Matrix
                                <button onclick=\"window.togglePingerFieldInfo(this,'pp-apdex',event)\" class="text-[var(--text-muted)] hover:text-[var(--accent-cyan)] focus:outline-none" title="Info">${INFO_ICON_SVG}</button>
                                ${buildPingerFieldPopover('pp-apdex')}
                            </span>
                            <span id="pinger-apdex" class="text-[var(--text-primary)] font-bold">--</span>
                        </li>

                        <li class="flex justify-between items-center py-1.5 border-b border-[var(--border-color)] whitespace-nowrap">
                            <span class="text-[var(--text-muted)] inline-flex items-center gap-1.5 relative">
                                Error Velocity
                                <button onclick=\"window.togglePingerFieldInfo(this,'pp-errvel',event)\" class="text-[var(--text-muted)] hover:text-[var(--accent-cyan)] focus:outline-none" title="Info">${INFO_ICON_SVG}</button>
                                ${buildPingerFieldPopover('pp-errvel')}
                            </span>
                            <span id="pinger-error-rate" class="text-rose-400 font-bold">--</span>
                        </li>

                        <li class="flex justify-between items-center py-1.5 border-b border-[var(--border-color)] whitespace-nowrap">
                            <span class="text-[var(--text-muted)] inline-flex items-center gap-1.5 relative">
                                P50 / P95 / P99
                                <button onclick=\"window.togglePingerFieldInfo(this,'pp-pct',event)\"" class="text-[var(--text-muted)] hover:text-[var(--accent-cyan)] focus:outline-none" title="Info">${INFO_ICON_SVG}</button>
                                ${buildPingerFieldPopover('pp-pct')}
                            </span>
                            <span id="pinger-percentiles" class="text-amber-400 font-bold">--</span>
                        </li>

                        <li class="flex justify-between items-center py-1.5 border-b border-[var(--border-color)] whitespace-nowrap">
                            <span class="text-[var(--text-muted)] inline-flex items-center gap-1.5 relative">
                                Status Signature
                                <button onclick=\"window.togglePingerFieldInfo(this,'pp-status',event)\"" class="text-[var(--text-muted)] hover:text-[var(--accent-cyan)] focus:outline-none" title="Info">${INFO_ICON_SVG}</button>
                                ${buildPingerFieldPopover('pp-status')}
                            </span>
                            <span id="pinger-reason" class="text-[var(--text-primary)] truncate pl-2 max-w-[150px]">--</span>
                        </li>

                        <li class="flex flex-col py-1.5 whitespace-nowrap">
                            <span class="text-[var(--text-muted)] text-[9px] tracking-widest mb-1 inline-flex items-center gap-1.5 uppercase relative">
                                Exception Output
                                <button onclick=\"window.togglePingerFieldInfo(this,'pp-exc',event)\"" class="text-[var(--text-muted)] hover:text-[var(--accent-cyan)] focus:outline-none" title="Info">${INFO_ICON_SVG}</button>
                                ${buildPingerFieldPopover('pp-exc')}
                            </span>
                            <span id="pinger-last-error" class="text-rose-400 text-[10px] bg-[var(--bg-base)] p-1.5 rounded border border-[var(--border-color)] break-all leading-snug whitespace-normal">--</span>
                        </li>
                    </ul>
                    </div>`;
        } else {
            detailsHTML = `
                <div class="h-full pr-2 pb-4">
                  <ul id="${id}-list" class="flex flex-col w-full mt-2"></ul>
                </div>`;
        }

        return `
        <div class="widget-main">
            <div class="flex justify-between items-center drag-handle cursor-move pb-3 border-b border-[var(--border-color)] mb-3 select-none">
                <h4 class="text-xs text-[var(--text-header)] font-mono tracking-widest font-bold flex items-center gap-2 uppercase">
                    ${titleStr}
                    <button onclick="window.openMetricInfo('${metric}', this, event)" class="text-[var(--text-muted)] hover:text-sky-400 focus:outline-none" title="Metric Info">
                        ${INFO_ICON_SVG}
                    </button>
                    <div id="popover-${metric}" class="hidden absolute left-0 top-6 w-[340px] bg-[var(--bg-widget)] border border-[var(--border-color)] shadow-2xl rounded-md p-5 font-sans text-xs text-[var(--text-primary)] normal-case leading-relaxed z-50">
                        <div class="flex justify-between items-center border-b border-[var(--border-color)] pb-2 mb-3">
                            <span class="font-mono text-[10px] tracking-widest text-[var(--accent-cyan)] font-bold uppercase">Telemetry Context</span>
                            <button onclick="document.getElementById('popover-${metric}').classList.add('hidden')" class="text-[var(--text-muted)] hover:text-[var(--text-header)] font-bold text-base leading-none focus:outline-none">&times;</button>
                        </div>
                        <div class="text-[var(--text-muted)] font-normal text-xs space-y-3">${TelemetryContexts[metric] || ''}</div>
                    </div>
                </h4>
                <div class="flex gap-2 items-center">
                    ${btnHtml}
                    <button onclick="window.engine.removeWidget('${id}')" class="text-[var(--text-muted)] hover:text-rose-500 text-lg leading-none font-bold focus:outline-none">&times;</button>
                </div>
            </div>
            <div class="flex-1 flex flex-col min-h-0 relative w-full overflow-hidden">
                ${contentHTML}
            </div>
        </div>
        <div id="${id}-details" class="widget-details-panel ${expandText === 'SHRINK' ? 'open' : ''}">
            ${detailsHTML}
        </div>
    `;
    },

    getPingerHTML: (id, detailState) => {
        return `
            <div class="flex flex-col flex-1 min-w-0 pr-2 h-full">
                <div class="flex items-center justify-between gap-3 flex-none font-mono mt-3">
                    <input type="text" id="ping-url" value="http://localhost:8080/api/v1/ping" placeholder="https://api.example/health" class="flex-1 bg-[var(--bg-base)] border border-[var(--border-color)] rounded px-3 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-[var(--accent-blue)] transition-colors shadow-inner">
                    <button id="ping-toggle" onclick="window.engine.togglePinger()" class="text-[10px] bg-[var(--bg-sidebar)] border border-[var(--border-color)] text-[var(--text-primary)] px-4 py-1.5 rounded hover:bg-[var(--border-color)] transition-colors font-bold uppercase tracking-widest focus:outline-none">START</button>
                </div>
                <div id="ping-url-warning" class="hidden mt-1 text-[10px] font-mono text-rose-400"></div>
                <div class="mt-2 text-[10px] font-mono text-[var(--text-muted)] flex items-start gap-1.5 leading-relaxed">
                    <span class="text-amber-500">▲</span>
                    <span>Client-side network probe active. Continuous loop requires viewport visibility and CORS access tokens.</span>
                </div>

                <div class="flex-1 flex flex-col justify-center items-center bg-[var(--bg-base)] rounded-lg border border-[var(--border-color)] mt-4 shadow-[inset_0_0_30px_rgba(0,0,0,0.8)] relative overflow-hidden">
                    <div class="absolute inset-0 bg-[radial-gradient(ellipse_at_center,_var(--tw-gradient-stops))] from-slate-800/20 via-transparent to-transparent" id="${id}-glow"></div>
                    <div class="z-10 flex flex-col items-center">
                        <span class="text-[10px] text-[var(--text-muted)] uppercase tracking-widest font-bold mb-2">System Resilience</span>
                        <div class="flex items-baseline gap-1">
                            <span id="real-resilience-score" class="text-7xl font-black font-mono tracking-tighter text-[var(--text-muted)] drop-shadow-md">---</span>
                            <span id="real-resilience-percent" class="text-3xl font-bold text-[var(--text-muted)]/60">%</span>
                        </div>
                        <div class="mt-3 flex items-center gap-2 bg-[var(--bg-sidebar)] px-3 py-1 rounded-full border border-[var(--border-color)]">
                            <div id="pinger-pulse" class="w-1.5 h-1.5 rounded-full bg-slate-500"></div>
                            <span id="real-ping-ms" class="text-[10px] font-mono text-[var(--text-muted)]">Latency: -- ms</span>
                        </div>
                    </div>
                </div>
            </div>`;
    },

    getChartHTML: (id, detailState) => {
        return `<div id="${id}" style="height: 100%; width: 100%; min-height: 220px;" class="min-w-0 flex-1"></div>`;
    },

    getLogHTML: (id, detailState) => {
        return `
        <div class="flex flex-col h-full w-full">
            <div class="flex items-center gap-2 pb-3 mb-2 border-b border-[var(--border-color)] flex-shrink-0">
                <input type="text" id="${id}-search" placeholder="Search logs..." oninput="window.engine.updateLogWidget()" 
                       class="flex-1 bg-[var(--bg-base)] border border-[var(--border-color)] rounded px-3 py-1.5 text-xs text-[var(--text-primary)] focus:border-[var(--accent-blue)] outline-none font-mono shadow-inner">
                
                <button id="${id}-pause-btn" onclick="window.engine.toggleLogPause('${id}')" 
                        class="px-3 py-1.5 bg-emerald-950/40 border border-emerald-900 text-emerald-400 rounded text-[10px] font-bold tracking-widest uppercase hover:bg-emerald-900/60 transition-colors focus:outline-none shadow-sm">LIVE</button>
                
                <div class="relative inline-block text-left source-dropdown-container">
                    <button id="${id}-source-btn" onclick="document.getElementById('${id}-source-popover').classList.toggle('hidden')" 
                            class="bg-[var(--bg-base)] text-[10px] font-mono text-[var(--accent-cyan)] outline-none border border-[var(--border-color)] rounded px-3 py-1.5 flex items-center justify-between min-w-[130px] shadow-sm hover:bg-[var(--bg-sidebar)] transition-colors focus:outline-none">
                        <span id="${id}-source-label">ALL LOGS</span>
                        <span class="ml-2 text-[var(--text-muted)]">▼</span>
                    </button>
                    <div id="${id}-source-popover" class="hidden absolute left-0 mt-1 w-56 bg-[var(--bg-widget)] border border-[var(--border-color)] shadow-2xl rounded-md z-[100] flex flex-col font-mono text-[10px] max-h-64 overflow-y-auto scrollbar-thin">
                        </div>
                </div>
                
                <button onclick="window.copyToClipboard('${id}-content', this)" 
                        class="p-1.5 bg-[var(--bg-sidebar)] border border-[var(--border-color)] rounded text-[var(--text-muted)] hover:text-[var(--text-primary)] transition-colors flex items-center justify-center aspect-square shadow-sm focus:outline-none" title="Copy">
                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 5H6a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2v-1M8 5a2 2 0 012-2h2a2 2 0 012 2m0 0h2a2 2 0 012 2v3m2 4H10m0 0l3-3m-3 3l3 3"></path></svg>
                </button>
            </div>
            <div id="${id}-content" class="flex-1 overflow-y-auto overflow-x-hidden font-mono text-[11px] text-[var(--text-muted)] space-y-1 scrollbar-thin pr-2">
                <div class="italic p-3 text-center mt-2">Awaiting streams...</div>
            </div>
        </div>`;
    },

    generatePolicyHTML: (policy, idx) => {
        const p = policy || {};
        const isGrpc = p.type === 'grpc';
        const isResource = p.type === 'resource';
        const typeOptions = ['http', 'sql', 'grpc', 'redis', 'mongo', 'kafka', 'rabbitmq', 'resource']
            .map(t => `<option value="${t}" ${p.type === t ? 'selected' : ''}>${t.toUpperCase()}</option>`).join('');

        const ec = Number(p.errorChance ?? p.error_chance ?? 0);
        const lc = Number(p.latencyChance ?? p.latency_chance ?? 0);
        const dc = p.dropConnection ?? p.drop_connection ?? false;

        const inputCls = 'w-full bg-[var(--bg-base)] border border-[var(--border-color)] rounded px-3 py-2 text-xs text-[var(--text-primary)] font-mono outline-none focus:border-[var(--accent-blue)]';
        const labelCls = 'text-[11px] text-[var(--text-muted)] font-mono font-bold uppercase tracking-wide';

        return `
        <div class="bg-[var(--bg-widget)] border border-[var(--border-color)] rounded-lg overflow-hidden">
            <div class="flex items-center gap-2 px-4 py-2 bg-[var(--bg-sidebar)] border-b border-[var(--border-color)]">
                <span class="text-[11px] text-[var(--accent-cyan)] font-bold font-mono uppercase tracking-wider bg-[var(--bg-base)] px-2 py-0.5 rounded border border-[var(--border-color)]">${p.type || 'NEW'}</span>
                <input type="text" value="${escapeHTML(p.name || '')}" oninput="window.BuilderEngine.updateField(${idx}, 'name', this.value)" placeholder="Name" class="flex-1 bg-transparent border-none p-0 text-xs text-[var(--text-primary)] font-mono font-bold outline-none">
                <select onchange="window.BuilderEngine.updateField(${idx}, 'type', this.value)" class="bg-[var(--bg-base)] border border-[var(--border-color)] rounded px-2 py-1 text-[11px] text-[var(--text-primary)] font-mono outline-none cursor-pointer">${typeOptions}</select>
                <button onclick="window.BuilderEngine.removePolicy(${idx})" class="text-[var(--text-muted)] hover:text-rose-400 font-bold text-base leading-none">&times;</button>
            </div>

            <div class="p-4 space-y-3">
                <div>
                    <label class="${labelCls} block mb-1">Target</label>
                    <input type="text" value="${escapeHTML(p.target || '')}" oninput="window.BuilderEngine.updateField(${idx}, 'target', this.value)" placeholder="e.g. /api/users, all, orders:db" class="${inputCls}">
                </div>

                <div class="border-t border-[var(--border-color)] pt-3">
                    <div class="text-[10px] text-[var(--accent-cyan)] font-bold font-mono uppercase tracking-wider mb-2">Error</div>
                    <div class="grid grid-cols-3 gap-2">
                        <label class="flex flex-col gap-1"><span class="${labelCls}">Chance</span><input type="number" min="0" max="1" step="0.1" value="${ec}" oninput="window.BuilderEngine.updateField(${idx}, 'errorChance', this.value)" class="${inputCls}"></label>
                        <label class="flex flex-col gap-1"><span class="${labelCls}">Code</span><input type="number" value="${p.errorCode ?? p.error_code ?? 500}" oninput="window.BuilderEngine.updateField(${idx}, 'errorCode', this.value)" class="${inputCls}"></label>
                        <label class="flex flex-col gap-1"><span class="${labelCls}">Body</span><input type="text" value="${escapeHTML(p.errorBody ?? p.error_body ?? '')}" oninput="window.BuilderEngine.updateField(${idx}, 'errorBody', this.value)" placeholder="optional" class="${inputCls}"></label>
                    </div>
                </div>

                <div class="border-t border-[var(--border-color)] pt-3">
                    <div class="text-[10px] text-[var(--accent-cyan)] font-bold font-mono uppercase tracking-wider mb-2">Latency</div>
                    <div class="grid grid-cols-2 gap-2">
                        <label class="flex flex-col gap-1"><span class="${labelCls}">Chance</span><input type="number" min="0" max="1" step="0.1" value="${lc}" oninput="window.BuilderEngine.updateField(${idx}, 'latencyChance', this.value)" class="${inputCls}"></label>
                        <label class="flex flex-col gap-1"><span class="${labelCls}">Duration</span><input type="text" value="${escapeHTML(p.latencyDuration ?? p.latency_duration ?? '500ms')}" oninput="window.BuilderEngine.updateField(${idx}, 'latencyDuration', this.value)" placeholder="500ms, 2s" class="${inputCls}"></label>
                    </div>
                    <label class="flex items-center gap-2 mt-2">
                        <input type="checkbox" ${dc ? 'checked' : ''} onchange="window.BuilderEngine.updateField(${idx}, 'dropConnection', this.checked)" class="h-4 w-4 rounded bg-[var(--bg-base)] border-[var(--border-color)] cursor-pointer">
                        <span class="text-xs text-rose-400 font-mono font-bold">Drop Connection</span>
                    </label>
                </div>

                ${isGrpc ? `
                <div class="border-t border-[var(--border-color)] pt-3">
                    <label class="flex flex-col gap-1">
                        <span class="${labelCls}">gRPC Mode</span>
                        <select onchange="window.BuilderEngine.updateField(${idx}, 'streamRollMode', this.value)" class="${inputCls} cursor-pointer">
                            <option value="stream" ${(p.streamRollMode ?? p.stream_roll_mode) === 'stream' ? 'selected' : ''}>Stream</option>
                            <option value="unary" ${(p.streamRollMode ?? p.stream_roll_mode) === 'unary' ? 'selected' : ''}>Unary</option>
                        </select>
                    </label>
                </div>` : ''}

                ${isResource ? `
                <div class="border-t border-[var(--border-color)] pt-3">
                    <div class="text-[10px] text-[var(--accent-cyan)] font-bold font-mono uppercase tracking-wider mb-2">Resource</div>
                    <div class="grid grid-cols-3 gap-2">
                        <label class="flex flex-col gap-1">
                            <span class="${labelCls}">RAM MB</span>
                            <input type="number" min="0" max="4096" value="${p.ramChunkMB ?? p.ram_chunk_mb ?? 0}" oninput="window.BuilderEngine.updateField(${idx}, 'ramChunkMB', this.value)" class="${inputCls}">
                        </label>
                        <label class="flex flex-col gap-1">
                            <span class="${labelCls}">Interval</span>
                            <input type="text" value="${escapeHTML(p.ramInterval ?? p.ram_interval ?? '')}" oninput="window.BuilderEngine.updateField(${idx}, 'ramInterval', this.value)" placeholder="5s" class="${inputCls}">
                        </label>
                        <label class="flex flex-col gap-1">
                            <span class="${labelCls}">CPU %</span>
                            <input type="number" min="0" max="100" value="${p.throttleThreshold ?? p.throttle_threshold ?? 0}" oninput="window.BuilderEngine.updateField(${idx}, 'throttleThreshold', this.value)" class="${inputCls}">
                        </label>
                    </div>
                </div>` : ''}
            </div>
        </div>`;
    }
};