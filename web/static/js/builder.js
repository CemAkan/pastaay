import { UITemplates } from './ui_templates.js';
import { API } from './api.js';

const pending = [];
const STUB_METHODS = ['updateGlobal', 'addPolicy', 'removePolicy', 'updateField', 'deployToEngine', 'rollback'];
if (!window.BuilderEngine) {
    window.BuilderEngine = STUB_METHODS.reduce((acc, m) => {
        acc[m] = (...args) => pending.push([m, args]);
        return acc;
    }, {});
}

const BuilderEngine = (function () {
    // State
    let state = {
        version: 1,
        warmup_duration: '5s',
        enable_default_ignored: true,
        policies: [],
    };
    let history = [];
    let lintTimer = null;
    let historyTimer = null;

    const clone = () => JSON.parse(JSON.stringify(state));

    const pushHistory = () => {
        const snap = clone();
        const last = history[history.length - 1];
        if (last && JSON.stringify(last) === JSON.stringify(snap)) return;
        history.push(snap);
        if (history.length > 30) history.shift();
    };

    const scheduleHistory = () => {
        clearTimeout(historyTimer);
        historyTimer = setTimeout(pushHistory, 350);
    };

    // Duration
    const durationFromNs = (v) => {
        const ns = Number(v);
        if (!Number.isFinite(ns) || ns < 0) return null;
        if (ns === 0) return '0s';
        if (ns % 1e9 === 0) return `${ns / 1e9}s`;
        if (ns % 1e6 === 0) return `${ns / 1e6}ms`;
        if (ns % 1e3 === 0) return `${ns / 1e3}us`;
        return `${ns}ns`;
    };

    const normDur = (v, fb = '0s') => {
        if (v === null || v === undefined || v === '') return fb;
        if (typeof v === 'number') return durationFromNs(v) || fb;
        const s = String(v);
        if (/^\d+$/.test(s)) return durationFromNs(s) || fb;
        return s;
    };

    const normBool = (v, fb = false) => {
        if (v === null || v === undefined) return fb;
        return v === true || v === 'true';
    };

    // YAML fallback
    const scalarYAML = (v) => {
        if (v === null || v === undefined) return 'null';
        if (typeof v === 'number' || typeof v === 'boolean') return String(v);
        const s = String(v);
        if (s === '') return "''";
        const plain = /^[A-Za-z_./-][A-Za-z0-9_./:-]*$/.test(s)
            && !/^(true|false|null|undefined|yes|no|on|off|nan|inf)$/i.test(s);
        return plain ? s : JSON.stringify(s);
    };

    const isObj = (v) => v && typeof v === 'object' && !Array.isArray(v);

    const yamlPair = (k, v, indent) => {
        const pad = ' '.repeat(indent);
        if (Array.isArray(v) || isObj(v)) {
            const nested = yamlNode(v, indent + 2);
            if (nested === '[]' || nested === '{}') return `${pad}${k}: ${nested}`;
            return `${pad}${k}:\n${nested}`;
        }
        return `${pad}${k}: ${scalarYAML(v)}`;
    };

    const yamlNode = (v, indent = 0) => {
        const pad = ' '.repeat(indent);
        if (Array.isArray(v)) {
            if (!v.length) return '[]';
            return v.map(item => {
                if (!isObj(item)) return `${pad}- ${scalarYAML(item)}`;
                const ents = Object.entries(item);
                if (!ents.length) return `${pad}- {}`;
                const [fk, fv] = ents[0];
                const first = yamlPair(fk, fv, indent + 2).trimStart();
                const lines = [`${pad}- ${first}`];
                ents.slice(1).forEach(([k, val]) => lines.push(yamlPair(k, val, indent + 2)));
                return lines.join('\n');
            }).join('\n');
        }
        if (isObj(v)) {
            const ents = Object.entries(v);
            if (!ents.length) return '{}';
            return ents.map(([k, val]) => yamlPair(k, val, indent)).join('\n');
        }
        return `${pad}${scalarYAML(v)}`;
    };

    const dumpYAML = (v) => {
        if (window.jsyaml && typeof window.jsyaml.dump === 'function') {
            try { return window.jsyaml.dump(v, { indent: 2, lineWidth: -1 }); }
            catch (e) { console.warn('jsyaml.dump failed, using fallback:', e); }
        }
        return `${yamlNode(v)}\n`;
    };

    // DOM render
    const renderGlobals = () => {
        const w = document.getElementById('warmup-duration');
        if (w) w.value = state.warmup_duration;
        const i = document.getElementById('enable-default-ignored');
        if (i) i.checked = Boolean(state.enable_default_ignored);
    };

    const renderUI = () => {
        const c = document.getElementById('policies-container');
        if (!c) return;
        renderGlobals();

        if (state.policies.length === 0) {
            c.innerHTML = '<div class="text-center p-8 border border-dashed border-[#30363d] rounded-xl text-zinc-500 text-xs font-mono bg-[#0d1117]/50 shadow-inner">No injector units configured. Add a unit to begin.</div>';
            updateYAML();
            return;
        }

        try {
            c.innerHTML = state.policies.map((p, i) => UITemplates.generatePolicyHTML(p, i)).join('');
        } catch (e) {
            console.error('Policy render failed:', e);
            c.innerHTML = `<div class="text-rose-400 text-xs font-mono p-3 border border-rose-700/40 rounded">Render error: ${String(e.message || e)}</div>`;
        }
        updateYAML();
    };

    // State sync with backend
    const init = () => {
        if (!document.getElementById('policies-container')) return;

        try {
            pushHistory();
            renderUI();
        } catch (e) {
            console.error('Initial render failed:', e);
        }

        API.fetchState().then(s => {
            if (!s || typeof s !== 'object') return;

            state.version = parseInt(s.version ?? s.Version ?? state.version, 10) || 1;
            state.warmup_duration = normDur(
                s.warmup_duration ?? s.warmupDuration ?? s.WarmupDuration,
                state.warmup_duration
            );
            state.enable_default_ignored = normBool(
                s.enable_default_ignored ?? s.enableDefaultIgnored ?? s.EnableDefaultIgnored,
                state.enable_default_ignored
            );

            const rawPolicies = Array.isArray(s.policies) ? s.policies.filter(Boolean) : [];
            state.policies = rawPolicies.map(p => ({
                name: p.name || '',
                type: p.type || 'http',
                target: p.target || 'all',
                latencyChance: Number(p.latency_chance ?? p.latencyChance ?? 0) || 0,
                latencyDuration: normDur(p.latency_duration ?? p.latencyDuration, '0s'),
                errorChance: Number(p.error_chance ?? p.errorChance ?? 0) || 0,
                errorCode: Number(p.error_code ?? p.errorCode ?? 0) || 0,
                errorBody: p.error_body ?? p.errorBody ?? '',
                dropConnection: normBool(p.drop_connection ?? p.dropConnection, false),
                streamRollMode: p.stream_roll_mode ?? p.streamRollMode ?? '',
                throttleThreshold: Number(p.throttle_threshold ?? p.throttleThreshold ?? 0) || 0,
                ramChunkMB: Number(p.ram_chunk_mb ?? p.ramChunkMB ?? 0) || 0,
                ramInterval: normDur(p.ram_interval ?? p.ramInterval, ''),
            }));

            pushHistory();
            renderUI();
        }).catch(err => console.warn('State fetch failed, using defaults:', err));

        fetch('/console/api/discover', { headers: { 'X-Pastaay-Token': API.getToken() } }).then(r => r.json()).then(t => {
            const list = document.getElementById('discovered-targets');
            if (Array.isArray(t) && list) {
                list.innerHTML = t.map(x => `<option value="${x}">`).join('');
            }
        }).catch(() => {});
    };

    // Public mutators
    const updateGlobal = (k, v) => {
        state[k] = k === 'enable_default_ignored' ? normBool(v) : v;
        updateYAML();
        scheduleHistory();
    };

    const addPolicy = () => {
        state.policies.push({
            name: 'policy-' + Math.floor(Math.random() * 1000),
            type: 'http',
            target: 'all',
            latencyChance: 0,
            latencyDuration: '0s',
            errorChance: 0,
            errorCode: 0,
            errorBody: '',
            dropConnection: false,
        });
        renderUI();
        pushHistory();
    };

    const removePolicy = (i) => {
        state.policies.splice(i, 1);
        renderUI();
        pushHistory();
    };

    const updateField = (i, f, v) => {
        const p = state.policies[i];
        if (!p) return;

        if (f.endsWith('Chance')) {
            v = parseFloat(v) || 0;
        } else if (f === 'errorCode' || f === 'throttleThreshold' || f === 'ramChunkMB') {
            v = parseInt(v, 10) || 0;
        } else if (f === 'dropConnection') {
            v = v === true || v === 'true';
        }
        p[f] = v;

        if (f === 'type') {
            if (p.type === 'grpc' && !p.streamRollMode) p.streamRollMode = 'stream';
            if (p.type === 'resource') {
                p.throttleThreshold = p.throttleThreshold ?? 0;
                p.ramChunkMB = p.ramChunkMB ?? 0;
                p.ramInterval = p.ramInterval ?? '';
            }
            if (p.type !== 'grpc') delete p.streamRollMode;
            if (p.type !== 'resource') {
                delete p.throttleThreshold;
                delete p.ramChunkMB;
                delete p.ramInterval;
            }
            renderUI();
        } else {
            updateYAML();
        }
        scheduleHistory();
    };

    // YAML emit + lint
    const getYAML = () => {
        const t = {
            version: state.version,
            warmup_duration: state.warmup_duration,
            enable_default_ignored: state.enable_default_ignored,
            policies: state.policies.map(p => {
                const out = {
                    name: p.name,
                    type: p.type,
                    target: p.target,
                    latency_chance: p.latencyChance,
                    latency_duration: p.latencyDuration,
                    error_chance: p.errorChance,
                    error_code: p.errorCode,
                    error_body: p.errorBody,
                    drop_connection: p.dropConnection,
                };
                if (p.type === 'grpc' && p.streamRollMode) out.stream_roll_mode = p.streamRollMode;
                if (p.type === 'resource') {
                    if (p.throttleThreshold) out.throttle_threshold = p.throttleThreshold;
                    if (p.ramChunkMB) out.ram_chunk_mb = p.ramChunkMB;
                    if (p.ramInterval) out.ram_interval = p.ramInterval;
                }
                return out;
            }),
        };
        return dumpYAML(t);
    };

    const updateYAML = () => {
        const y = getYAML();
        const pre = document.getElementById('yaml-preview');
        if (pre) {
            pre.textContent = y;
            if (typeof hljs !== 'undefined') {
                pre.removeAttribute('data-highlighted'); // clean highlight
                hljs.highlightElement(pre); // highlight again
            }
        }

        clearTimeout(lintTimer);
        lintTimer = setTimeout(() => {
            API.lintPlan(y).then(plan => {
                const formatted = {
                    status: plan?.status || ((plan?.total_risk || 0) > 0.6 ? 'HIGH' : 'SAFE'),
                    score: Math.min(Math.round((plan?.total_risk || 0) * 100), 100),
                    issues: plan?.issues || [],
                };
                UITemplates.updateLintUI(formatted);
            }).catch(e => console.warn('Lint failed:', e));
        }, 500);
    };

    // Deploy / rollback
    const deployToEngine = async (btn) => {
        if (!btn) return;
        const token = document.getElementById('webhook-token')?.value || sessionStorage.getItem('pastaay_token') || '';
        const orig = btn.innerHTML;
        const baseCls = 'bg-[#2c2f36] hover:bg-[#30363d] border border-[#30363d] text-white px-5 py-1.5 rounded text-[11px] font-mono font-bold tracking-widest shadow-sm';

        UITemplates.setBtnState(btn, 'COMMITTING...', 'bg-amber-600 text-white px-5 py-1.5 rounded text-[11px] font-mono font-bold shadow uppercase tracking-widest');
        btn.disabled = true;

        try {
            const r = await API.deployChaos(getYAML(), token);
            if (!r || !r.ok) throw new Error('Rejected');
            UITemplates.setBtnState(btn, '✔ DEPLOYED', 'bg-emerald-600 text-white px-5 py-1.5 rounded text-[11px] font-mono font-bold shadow uppercase tracking-widest');
                setTimeout(() => UITemplates.setBtnState(btn, orig, baseCls), 2000);
        } catch (e) {
            console.error('Deploy failed:', e);
            UITemplates.setBtnState(btn, 'NET ERROR', 'bg-rose-600 text-white px-5 py-1.5 rounded text-[11px] font-mono font-bold shadow uppercase tracking-widest');

            if (state.policies.length === 5 && !localStorage.getItem("ee_office")) {
                localStorage.setItem("ee_office", "1");
                btn.insertAdjacentHTML("afterend", "<span id=ee-stapler style=font-size:30px;margin-left:8px;animation:eeShake 0.2s ease-in-out 5>📎</span>");
                setTimeout(() => { const s = document.getElementById("ee-stapler"); if(s) s.remove(); }, 3000);
                setTimeout(() => window._eeLog("I believe you have my stapler.", "MILTON"), 800);
            }

            setTimeout(() => UITemplates.setBtnState(btn, orig, baseCls), 3000);
        } finally {
            btn.disabled = false;
        }
    };

    const rollback = () => {
        if (history.length <= 1) return;
        history.pop();
        state = JSON.parse(JSON.stringify(history[history.length - 1]));
        renderUI();
    };

    return { init, updateGlobal, addPolicy, removePolicy, updateField, deployToEngine, rollback };
})();

Object.assign(window.BuilderEngine, BuilderEngine);

const boot = () => {
    try {
        BuilderEngine.init();
    } catch (e) {
        console.error('BuilderEngine init failed:', e);
    }

    while (pending.length) {
        const [name, args] = pending.shift();
        try { window.BuilderEngine[name]?.(...args); }
        catch (e) { console.error(`Replay ${name} failed:`, e); }
    }
};

if (document.readyState === 'loading') {
    window.addEventListener('DOMContentLoaded', boot);
} else {
    boot();
}