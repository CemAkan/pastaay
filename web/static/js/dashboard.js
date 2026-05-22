import { PingerController } from './pinger.js';
import { TelemetryPipeline } from './telemetry.js';
import { WidgetManager } from './widget.js';
import { API } from './api.js';
import { UITemplates } from './ui_templates.js';

class DashboardEngine {
    constructor() {
        this.logs = [];
        this.activeFilter = 'all';
        this.activeFilterLabel = 'ALL LOGS';

        this.navStateL1 = null; // Pod
        this.navStateL2 = null; // Protocol

        this.widgets = new WidgetManager(this);
        this.pinger = new PingerController(this);
        this.telemetry = new TelemetryPipeline(this);

        try {
            this.widgets.initSortable();
            this.widgets.loadLayout();
            this.rebuildLogFilters();
            this.updateLogWidget();
            this.telemetry.start();
            this._setupLogDelegation();
            this._setupIdleEasterEggs();
        } catch (e) {
            console.error('Engine startup failed:', e);
        }
    }

    addLocalLog(msg, source = 'frontend') {
        this.logs.push({
            source,
            name: 'sys',
            msg,
            ts: Date.now(),
            _fe: true
        });
        if (this.logs.length > 600) this.logs.splice(0, this.logs.length - 600);
        this.rebuildLogFilters();
        this.updateLogWidget();
    }

    toggleWidgetExpand(id) { this.widgets.toggleExpand(id); }
    addWidget(metric, expanded = false) { this.widgets.add(metric, expanded); }
    removeWidget(id) { this.widgets.remove(id); }
    togglePinger() { this.pinger.toggle(); }
    updatePingerSetting(field, value) { this.pinger.updateSetting(field, value); }

    updateWidgetSource(widgetId, filterVal, labelText, closePopover = true) {
        this.activeFilter = filterVal;
        this.activeFilterLabel = labelText;

        const labelEl = document.getElementById(`${widgetId}-source-label`);
        if (labelEl) labelEl.textContent = labelText;

        if (closePopover) {
            const pop = document.getElementById(`${widgetId}-source-popover`);
            if (pop) pop.classList.add('hidden');
        }

        this.updateLogWidget();
    }

    _buildMenuTree() {
        const tree = {};
        this.logs.forEach(l => {
            const pod = l.source || 'unknown-pod';
            const protocol = l.name || 'sys';
            let method = 'General';

            try {
                const parsed = JSON.parse(l.msg);
                if (parsed.target) method = parsed.target;
            } catch(e) {}

            l._parsedMethod = method; // save for filtering

            if (!tree[pod]) tree[pod] = {};
            if (!tree[pod][protocol]) tree[pod][protocol] = new Set();
            tree[pod][protocol].add(method);
        });
        return tree;
    }

    syncBackendLogs(arr) {
        if (!Array.isArray(arr)) return;
        const kept = this.logs.filter(l => l._fe === true);

        const merged = arr.map(l => {
            if (!l) return null;
            return {
                source: l.source,
                name: l.name.replace('-chaos', '').replace('ritual-engine', 'http'),
                msg: String(l.msg || l.message || ''),
                ts: Number(l.ts) || Date.now(),
            };
        }).filter(Boolean);

        this.logs = [...kept, ...merged].sort((a, b) => a.ts - b.ts);

        //limit
        if (this.logs.length > 600) this.logs.splice(0, this.logs.length - 600);

        this.rebuildLogFilters();
        this.updateLogWidget();
    }

    rebuildLogFilters() {
        const tree = this._buildMenuTree();
        const logWidgets = this.widgets.list.filter(w => w.metric === 'log');

        logWidgets.forEach(w => {
            const popover = document.getElementById(`${w.id}-source-popover`);
            if (!popover) return;
            popover.innerHTML = '';

            const createBtn = (text, icon, onClick, isBack = false, isLeaf = false) => {
                const btn = document.createElement('button');
                let cls = `flex items-center text-left w-full px-4 py-2 hover:bg-[#2c2f36] transition-colors focus:outline-none `;
                if (isBack) cls += `text-amber-400 font-bold border-b border-[#30363d] sticky top-0 bg-[#1f2226] z-10`;
                else if (isLeaf) cls += `text-sky-400`;
                else cls += `text-slate-300`;

                btn.className = cls;
                btn.innerHTML = `<span class="w-5 text-center mr-1">${icon}</span> <span class="truncate">${text}</span>`;
                btn.onclick = (e) => { e.stopPropagation(); onClick(); };
                return btn;
            };

            // Pods / Hosts
            if (!this.navStateL1) {
                popover.appendChild(createBtn('ALL LOGS', '≡', () => this.updateWidgetSource(w.id, 'all', 'ALL LOGS', true), false, true));
                Object.keys(tree).forEach(pod => {
                    popover.appendChild(createBtn(pod, '▶', () => { this.navStateL1 = pod; this.rebuildLogFilters(); }));
                });
            }
            // Protocols under a Pod
            else if (this.navStateL1 && !this.navStateL2) {
                popover.appendChild(createBtn('BACK', '←', () => { this.navStateL1 = null; this.rebuildLogFilters(); }, true));
                popover.appendChild(createBtn(`ALL ${this.navStateL1}`, '≡', () => this.updateWidgetSource(w.id, `pod:${this.navStateL1}`, this.navStateL1, true), false, true));

                Object.keys(tree[this.navStateL1] || {}).forEach(protocol => {
                    popover.appendChild(createBtn(protocol.toUpperCase(), '▶', () => { this.navStateL2 = protocol; this.rebuildLogFilters(); }));
                });
            }
            // Methods under a Protocol
            else if (this.navStateL1 && this.navStateL2) {
                popover.appendChild(createBtn('BACK', '←', () => { this.navStateL2 = null; this.rebuildLogFilters(); }, true));
                popover.appendChild(createBtn(`ALL ${this.navStateL2}`, '≡', () => this.updateWidgetSource(w.id, `proto:${this.navStateL1}:${this.navStateL2}`, `${this.navStateL1} > ${this.navStateL2.toUpperCase()}`, true), false, true));

                const methods = tree[this.navStateL1][this.navStateL2] || new Set();
                methods.forEach(method => {
                    popover.appendChild(createBtn(method, '•', () => {
                        this.updateWidgetSource(w.id, `method:${this.navStateL1}:${this.navStateL2}:${method}`, method, true);
                    }, false, true));
                });
            }
        });
    }

    updateLogWidget() {
        const logWidgets = this.widgets.list.filter(w => w.metric === 'log');
        logWidgets.forEach(w => {
            if (w.isPaused) return;

            const box = document.getElementById(`${w.id}-content`);
            if (!box) return;

            const searchInput = document.getElementById(`${w.id}-search`);
            const query = searchInput ? searchInput.value.toLowerCase() : '';
            const filter = this.activeFilter || 'all';

            let filtered = this.logs;

            // Filtering
            if (filter !== 'all') {
                if (filter.startsWith('pod:')) {
                    const pod = filter.split(':')[1];
                    filtered = this.logs.filter(l => l.source === pod);
                } else if (filter.startsWith('proto:')) {
                    const [_, pod, proto] = filter.split(':');
                    filtered = this.logs.filter(l => l.source === pod && l.name === proto);
                } else if (filter.startsWith('method:')) {
                    const parts = filter.split(':');
                    const pod = parts[1];
                    const proto = parts[2];
                    const method = parts.slice(3).join(':');
                    filtered = this.logs.filter(l => l.source === pod && l.name === proto && l._parsedMethod === method);
                }
            }

            if (query) {
                filtered = filtered.filter(l => l.msg.toLowerCase().includes(query));
            }

            if (filtered.length === 0) {
                box.innerHTML = `<div class="text-slate-600 italic p-3 text-center mt-2">No payload streams detected for this scope.</div>`;
                return;
            }

            const nearBottom = (box.scrollHeight - box.scrollTop - box.clientHeight) < 40;
            box.innerHTML = filtered.map(l => this._renderLogLine(l, w.id)).join('');
            if (nearBottom) box.scrollTop = box.scrollHeight;
        });
    }

    _renderLogLine(l, widgetId) {
        if (!this._podColors) {
            this._podColors = {};
            this._podPalette = ['text-cyan-400','text-emerald-400','text-amber-400','text-purple-400',
                'text-rose-400','text-blue-400','text-lime-400','text-yellow-400','text-orange-400',
                'text-pink-400','text-teal-400','text-indigo-400','text-fuchsia-400','text-sky-400'];
            this._podNextColor = 0;
        }
        if (!this._podColors[l.source]) {
            this._podColors[l.source] = this._podPalette[this._podNextColor++ % this._podPalette.length];
        }
        const podColor = this._podColors[l.source];

        const protocolColors = {
            'http': 'text-emerald-400', 'grpc': 'text-cyan-400', 'sql': 'text-amber-400',
            'redis': 'text-rose-400', 'kafka': 'text-purple-400', 'rabbitmq': 'text-yellow-400',
            'mongo': 'text-lime-400', 'resource': 'text-orange-400'
        };
        const proto = l.name.toLowerCase();
        const methodColor = protocolColors[proto] || 'text-slate-400';
        const label = `<span class="${podColor}">${l.source}</span> · <span class="${methodColor}">${l.name.toUpperCase()}</span>`;

        let msg = l.msg;
        let isError = false;
        let isWarn = false;

        try {
            const obj = JSON.parse(l.msg);
            if (obj.level === "ERROR") isError = true;
            if (obj.level === "WARN") isWarn = true;
            msg = `[${obj.protocol || 'sys'}] ${obj.message}`;
        } catch(e) {
            if (msg.includes('ERROR') || msg.includes('FATAL')) isError = true;
            else if (msg.includes('WARN')) isWarn = true;
        }

        let escapedMsg = UITemplates.escapeHTML(msg);
        if (isError) escapedMsg = `<span class="text-rose-500 font-bold">${escapedMsg}</span>`;
        else if (isWarn) escapedMsg = `<span class="text-amber-500">${escapedMsg}</span>`;

        const b64Msg = btoa(encodeURIComponent(l.msg).replace(/%([0-9A-F]{2})/g, (_, p1) => String.fromCharCode('0x' + p1)));

        return `<div data-log="${b64Msg}" data-widget="${widgetId}" 
                 class="log-line hover:bg-sky-500/10 cursor-pointer px-2 py-1 text-[11px] rounded transition-colors border-l-2 border-transparent hover:border-sky-500 flex items-center gap-2 min-w-0">
            <span class="font-bold shrink-0 font-mono">[${label}]</span> 
            <span class="text-slate-300 truncate">${escapedMsg}</span>
        </div>`;
    }

    _setupLogDelegation() {
        document.getElementById('dashboard-grid').addEventListener('click', (e) => {
            const line = e.target.closest('.log-line');
            if (!line) return;
            const b64Msg = line.dataset.log;
            const widgetId = line.dataset.widget;
            if (b64Msg && widgetId) this.selectLogLine(line, widgetId);
        });
    }

    _setupIdleEasterEggs() {
        let t;
        const r = () => { clearTimeout(t); t = setTimeout(() => {
            if (!localStorage.getItem("ee_donnie")) {
                localStorage.setItem("ee_donnie", "1");
                document.body.insertAdjacentHTML("beforeend", "<span id=ee-frank style=position:fixed;top:20px;right:20px;font-size:50px;z-index:9999>🐰</span>");
                setTimeout(() => { const f = document.getElementById("ee-frank"); if(f) f.remove(); }, 3000);
                if (window._eeLog) window._eeLog("28 days, 6 hours, 42 minutes, 12 seconds.", "FRANK");
            }
        }, 28000); };
        document.addEventListener("mousemove", r); document.addEventListener("keydown", r); r();
    }


    selectLogLine(el, id) {
        const b64Msg = el.dataset.log;
        if (!b64Msg) return;

        const analysisPanel = document.getElementById(`${id}-analysis-content`);
        if (!analysisPanel) return;

        const msg = decodeURIComponent(atob(b64Msg).split('').map(c => '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2)).join(''));
        let parsedObj = {};
        let displayMsg = msg;
        let level = 'INFO';

        try {
            parsedObj = JSON.parse(msg);
            if (parsedObj.payload && typeof parsedObj.payload === 'string') {
                try { parsedObj.payload = JSON.parse(parsedObj.payload); } catch(e) {}
            }
            displayMsg = JSON.stringify(parsedObj, null, 2);
            if (parsedObj.level) level = parsedObj.level;
        } catch(e) {
            if (msg.includes('ERROR')) level = 'ERROR';
        }

        let highlightedCode = displayMsg;
        if (typeof hljs !== 'undefined') {
            try { highlightedCode = hljs.highlight(displayMsg, { language: 'json' }).value; } catch(hErr){}
        }

        const traceId = parsedObj?.trace_id || '';
        const spanId = parsedObj?.span_id || '';
        const tsRaw = parsedObj?.timestamp || new Date().toISOString();
        const tsDate = new Date(tsRaw);
        const timestamp = isNaN(tsDate.getTime()) ? new Date().toLocaleString() :
            tsDate.toLocaleDateString('en-US', {month:'short',day:'numeric'}) + ' ' +
            tsDate.toLocaleTimeString('en-US', {hour12:false, hour:'2-digit',minute:'2-digit',second:'2-digit'}) +
            '.' + String(tsDate.getMilliseconds()).padStart(3,'0');

        let lvlColor = 'text-sky-400 border-sky-400/30 bg-sky-400/10';
        if (level === 'ERROR') lvlColor = 'text-rose-400 border-rose-400/30 bg-rose-400/10';

        const jaegerButton = traceId ?
            `<button onclick="window.open('http://localhost:16686/trace/${traceId}', '_blank')" class="text-[9px] bg-[#0d1117] border border-[#30363d] px-2 py-1 rounded hover:text-sky-400 text-slate-300 transition-colors shadow-sm focus:outline-none">View in Jaeger ↗</button>` : '';

        analysisPanel.innerHTML = `
        <div class="flex items-center gap-2 mb-4">
            <span class="px-2 py-0.5 rounded text-[10px] font-bold border ${lvlColor}">${level}</span>
            <span class="text-slate-500 text-[10px]">${timestamp}</span>
        </div>
        <div class="grid grid-cols-2 gap-2 mb-4 text-[10px]">
            <div class="bg-[#0d1117] p-2 rounded border border-[#30363d] shadow-inner">
                <span class="text-slate-500 block mb-1">Trace ID</span>
                <span class="text-slate-300 font-mono break-all">${traceId || 'N/A'}</span>
            </div>
            <div class="bg-[#0d1117] p-2 rounded border border-[#30363d] shadow-inner">
                <span class="text-slate-500 block mb-1">Span ID</span>
                <span class="text-slate-300 font-mono break-all">${spanId || 'N/A'}</span>
            </div>
        </div>
        <div class="mb-2 flex justify-between items-center">
            <span class="text-[10px] text-slate-400 uppercase font-bold">Structured Payload</span>
            ${jaegerButton}
        </div>
        <pre class="text-[10px] font-mono bg-[#0f172a] p-3 rounded text-slate-300 whitespace-pre-wrap break-words border border-[#30363d] shadow-inner">${highlightedCode}</pre>
        <div id="${id}-ee-lynch" class="hidden"></div>`;

        // Lynchian
        const eeKey = `ee_lynch_${id}`;
        if (this._lynchTimer) clearTimeout(this._lynchTimer);
        if (!localStorage.getItem(eeKey)) {
            this._lynchTimer = setTimeout(() => {
                const el = document.getElementById(`${id}-ee-lynch`);
                if (el && analysisPanel && analysisPanel.parentElement && !analysisPanel.parentElement.classList.contains('hidden')) {
                    localStorage.setItem(eeKey, '1');
                    el.innerHTML = `<div class="mt-3 p-3 border border-red-900/40 rounded bg-gradient-to-r from-red-950/30 to-transparent"><span class="text-[9px] text-red-400 uppercase font-bold block mb-1">THE ARM</span><span class="text-red-300/70 text-[10px] font-mono italic">Sometimes my arms bend back...</span></div>`;
                    el.classList.remove('hidden');
                    setTimeout(() => el.classList.add('hidden'), 5000);
                }
            }, 120000);
        }
    }

    toggleLogPause(id) {
        const w = this.widgets.list.find(x => x.id === id);
        if (!w) return;
        w.isPaused = !w.isPaused;
        const btn = document.getElementById(`${id}-pause-btn`);
        if (btn) {
            btn.textContent = w.isPaused ? "PAUSED" : "LIVE";
            btn.className = w.isPaused ? "px-3 py-1.5 bg-rose-950/40 border border-rose-900 text-rose-400 rounded text-[10px] font-bold tracking-widest uppercase focus:outline-none" : "px-3 py-1.5 bg-emerald-950/40 border border-emerald-900 text-emerald-400 rounded text-[10px] font-bold tracking-widest uppercase focus:outline-none";
        }
    }
}

window.DashboardEngine = DashboardEngine;

// Global log
window._openLogDetail = function(el, id) {
    if (window.engine) window.engine.selectLogLine(el, id);
};

const initDashboard = () => {
    if (document.getElementById('dashboard-grid') && !window.engine) {
        window.engine = new DashboardEngine();
        window.dispatchEvent(new CustomEvent('engineReady'));
    }
};

if (document.readyState === 'loading') {
    window.addEventListener('DOMContentLoaded', initDashboard);
} else {
    initDashboard();
}

window.abortAllChaos = function () {
    const wasDeploying = window._ee_shaun_flag;
    window._ee_shaun_flag = false; // consume
    API.abortChaos().then(() => {
        if (window.engine) {
            window.engine.addLocalLog('Emergency reset executed', 'system');
            // Shaun of the Dead
            if (wasDeploying && !localStorage.getItem('ee_shaun')) {
                localStorage.setItem('ee_shaun', '1');
                window._eeLog("You've got red on you.", 'ED');
            }
        }
    });
};