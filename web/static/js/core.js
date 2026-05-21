window.Pastaay = window.Pastaay || {};
window.lastMetrics = window.lastMetrics || {};
window.lastTime = window.lastTime || Date.now();
window.currentRawYaml = window.currentRawYaml || "version: 1\npolicies: []";
window.activePoliciesCache = window.activePoliciesCache || [];

window.openModal = function(id) { document.getElementById(id).classList.remove('hidden'); }
window.closeModal = function(id) { document.getElementById(id).classList.add('hidden'); }

window.showInfoDialog = function(title, contentHTML) {
    // Info dialog handled by metric popovers
}

window.closeInfoDialog = function() {
    ['pinger', 'impact_matrix', 'global_rate', 'log'].forEach(m => {
        const pop = document.getElementById(`popover-${m}`);
        if(pop) pop.classList.add('hidden');
    });
}

window.openYamlModal = function() {
    const preview = document.getElementById('raw-yaml-content');
    if (preview) {
        preview.removeAttribute('data-highlighted');
        preview.textContent = window.currentRawYaml || "version: 1\npolicies: []";
        if(typeof hljs !== 'undefined') {
            try { hljs.highlightElement(preview); } catch(e){}
        }
    }
    window.openModal('yaml-modal');
}

window.toggleSensors = function() {
    const btn = document.querySelector('button[onclick="window.toggleSensors()"]');
    const panel = document.getElementById('sensors-popover');
    if(panel) panel.classList.toggle('hidden');
    const p = document.getElementById('add-panel-popover'); if(p) p.classList.add('hidden');
    const active = panel && !panel.classList.contains('hidden');
    if(btn) {
        btn.classList.toggle('text-slate-200', active);
        btn.classList.toggle('text-slate-400', !active);
    }
}
window.toggleAddPanel = function() {
    const btn = document.querySelector('button[onclick="window.toggleAddPanel()"]');
    const panel = document.getElementById('add-panel-popover');
    if(panel) panel.classList.toggle('hidden');
    const s = document.getElementById('sensors-popover'); if(s) s.classList.add('hidden');
    const active = panel && !panel.classList.contains('hidden');
    if(btn) {
        btn.classList.toggle('ring-1', active);
        btn.classList.toggle('ring-sky-500/40', active);
    }
}

window.togglePingerFieldInfo = function(btn, fieldId) {
    const pop = document.getElementById('popover-' + fieldId);
    if (!pop) return;
    document.querySelectorAll('[id^="popover-pinger-field-"]').forEach(p => {
        if (p.id !== 'popover-' + fieldId) p.classList.add('hidden');
    });
    const wasHidden = pop.classList.contains('hidden');
    pop.classList.toggle('hidden');
    if (wasHidden && !pop.classList.contains('hidden')) {
        const r = btn.getBoundingClientRect();
        pop.style.position = 'fixed';
        pop.style.left = Math.min(r.left, window.innerWidth - 300) + 'px';
        pop.style.top = Math.min(r.bottom + 4, window.innerHeight - 180) + 'px';
        pop.style.width = '280px';
        pop.style.zIndex = '200';
    }
};

window.addEventListener('click', function(e) {
    if (!e.target.closest('button[onclick="window.toggleSensors()"]') && !e.target.closest('#sensors-popover')) {
        const p1 = document.getElementById('sensors-popover'); if(p1) p1.classList.add('hidden');
        const b1 = document.querySelector('button[onclick="window.toggleSensors()"]');
        if(b1) { b1.classList.remove('text-slate-200'); b1.classList.add('text-slate-400'); }
    }
    if (!e.target.closest('button[onclick="window.toggleAddPanel()"]') && !e.target.closest('#add-panel-popover')) {
        const p2 = document.getElementById('add-panel-popover'); if(p2) p2.classList.add('hidden');
        const b2 = document.querySelector('button[onclick="window.toggleAddPanel()"]');
        if(b2) { b2.classList.remove('ring-1', 'ring-sky-500/40'); }
    }
    if (!e.target.closest('button[onclick^="window.openMetricInfo"]') && !e.target.closest('[id^="popover-"]')) {
        ['pinger', 'impact_matrix', 'global_rate', 'log'].forEach(m => {
            const pop = document.getElementById(`popover-${m}`);
            if(pop) pop.classList.add('hidden');
        });
    }
    if (!e.target.closest('button[onclick^="window.togglePingerFieldInfo"]') && !e.target.closest('[id^="popover-pinger-field-"]')) {
        document.querySelectorAll('[id^="popover-pinger-field-"]').forEach(p => p.classList.add('hidden'));
    }
});

window.copyToClipboard = function(elementId, btn) {
    const el = document.getElementById(elementId);
    if(!el) return;

    navigator.clipboard.writeText(el.textContent);

    const originalContent = btn.innerHTML;
    btn.innerHTML = `<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path></svg>`;
    btn.classList.add('success');

    setTimeout(() => {
        btn.innerHTML = originalContent;
        btn.classList.remove('success');
    }, 1500);
}

window.formatNsToMs = function(nsVal) {
    if (!nsVal) return "0ms";
    if (typeof nsVal === 'string' && (nsVal.includes('s') || nsVal.includes('m') || nsVal.includes('ms'))) return nsVal;
    let ns = parseInt(nsVal, 10);
    return isNaN(ns) ? nsVal : (ns / 1000000) + "ms";
}

window.oracleYamlCache = "";
window.toggleOracle = function() { document.getElementById('oracle-window').classList.toggle('hidden'); }

window.updateOracleModelPlaceholder = function() {
    const provider = document.getElementById('oracle-provider')?.value || 'openai';
    const modelInput = document.getElementById('oracle-model');
    if (!modelInput) return;
    const defaults = { openai: 'gpt-4o-mini', deepseek: 'deepseek-reasoner', gemini: 'gemini-2.5-flash', anthropic: 'claude-sonnet-4-6' };
    modelInput.placeholder = defaults[provider] || 'Model Override';
};

// Init placeholder on DOM ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => window.updateOracleModelPlaceholder());
} else {
    window.updateOracleModelPlaceholder();
}

window.clearOracle = function() {
    const chat = document.getElementById('oracle-chat');
    if(chat) chat.innerHTML = `<div class="text-slate-500 font-mono">SYS ┠ Chat history cleared. Standing by.</div>`;
    window.oracleYamlCache = "";
    document.getElementById('oracle-action-panel')?.classList.add('hidden');
    const v = document.getElementById('oracle-yaml-preview');
    if(v) v.textContent = "Awaiting generation...";
}

window.sendOraclePrompt = function() {
    const input = document.getElementById('oracle-prompt'); const chat = document.getElementById('oracle-chat');
    const key = document.getElementById('oracle-key').value; const provider = document.getElementById('oracle-provider').value;
    const model = document.getElementById('oracle-model').value; const btn = document.getElementById('oracle-btn');
    const intensity = document.getElementById('oracle-intensity').value;

    if(!key) { chat.innerHTML += `<div class="mt-2 text-rose-500 font-mono">SYS ┠ Critical: Key unauthorized.</div>`; chat.scrollTop = chat.scrollHeight; return; }

    let queryText = input.value.trim() || "Autonomous Scan: Target highest loaded components.";
    input.value = ''; btn.disabled = true; btn.innerText = "...";

    chat.innerHTML += `<div class="mt-4 border-l-2 border-slate-600 pl-3 font-mono"><span class="text-slate-500 text-[9px] block mb-1 uppercase tracking-widest font-bold">User Query [DEFCON: ${intensity.toUpperCase()}]</span><span class="text-slate-300">${queryText}</span></div>`;
    chat.scrollTop = chat.scrollHeight;

    const ctx = typeof window.currentRawYaml !== 'undefined' ? `YAML:\n${window.currentRawYaml}\nMETRICS:\n${JSON.stringify(window.lastMetrics)}` : "Context dry";
    const authToken = document.getElementById('webhook-token')?.value || localStorage.getItem('pastaay_token') || '';

    fetch('/console/api/oracle', { method: 'POST', headers: { 'Content-Type': 'application/json','X-Pastaay-Token': authToken }, body: JSON.stringify({ provider, key, model, intensity, prompt: queryText, context: ctx }) })
        .then(r => r.json()).then(data => {
        if(data.error) throw new Error(data.error);

        chat.innerHTML += `<div class="mt-4 border-l-2 border-sky-500 pl-3 font-mono"><span class="text-sky-500 text-[9px] block mb-1 uppercase tracking-widest font-bold">Oracle Agent</span><span class="text-slate-300">Strategy synthesized. Validate blueprint context.</span></div>`;
        window.oracleYamlCache = data.yaml;

        const preview = document.getElementById('oracle-yaml-preview');
        if(preview) {
            preview.removeAttribute('data-highlighted');
            preview.textContent = data.yaml;
            hljs.highlightElement(preview);
        }

        document.getElementById('oracle-action-panel')?.classList.remove('hidden');
    })
        .catch(err => {
            chat.innerHTML += `<div class="mt-4 border-l-2 border-rose-500 pl-3 font-mono"><span class="text-rose-500 text-[9px] block mb-1 uppercase tracking-widest font-bold">Fatal Error</span><span class="text-rose-400 break-words whitespace-pre-wrap">${err.message}</span></div>`;
        })
        .finally(() => {
            chat.scrollTop = chat.scrollHeight;
            btn.disabled = false; btn.innerText = "Run";
        });
}

window.deployOracleConfig = function(btn) {
    if(!window.oracleYamlCache) return;
    const originalText = btn.innerText;
    btn.innerText = "DEPLOYING..."; btn.disabled = true;
    fetch('/chaos/webhook', { method: 'POST', headers: { 'Content-Type': 'application/yaml' }, body: window.oracleYamlCache })
        .then(r => {
            if(r.ok) { btn.parentElement.innerHTML = '<span class="text-emerald-500 font-mono text-[10px] font-bold">✔ SYNCED TO FLEET</span>'; window.oracleYamlCache = ""; }
            else { btn.innerText = "REJECTED"; btn.disabled = false; }
        }).catch(() => { btn.innerText = "NET ERROR"; btn.disabled = false; });
}

window.discardOracleConfig = function() {
    const preview = document.getElementById('oracle-yaml-preview'); if(preview) preview.textContent = "Awaiting generation...";
    document.getElementById('oracle-action-panel')?.classList.add('hidden');
    const chat = document.getElementById('oracle-chat');
    if(chat) {
        chat.innerHTML += `<div class="mt-3 text-slate-500 font-mono">SYS ┠ Payload discarded.</div>`;
        chat.scrollTop = chat.scrollHeight;
    }
    window.oracleYamlCache = "";
}

window.formatJson = function(el) {
    const parent = el.parentElement;
    const raw = parent.querySelector('span.text-slate-300').innerText;
    try {
        const obj = JSON.parse(raw);
        parent.querySelector('span.text-slate-300').innerHTML =
            `<pre class="bg-[#0d1117] p-2 rounded mt-1 border border-[#30363d] text-sky-300 text-[10px] whitespace-pre-wrap">${JSON.stringify(obj, null, 2)}</pre>`;
        el.style.display = 'none';
    } catch(e) {}
}

window.initTheme = function() {
    const savedTheme = localStorage.getItem('theme') || 'dark';
    document.documentElement.setAttribute('data-theme', savedTheme);
};

window.toggleTheme = function() {
    const current = document.documentElement.getAttribute('data-theme');
    const target = current === 'dark' ? 'light' : 'dark';
    document.documentElement.setAttribute('data-theme', target);
    localStorage.setItem('theme', target);
};

window.initTheme();

window._eeLog = function(msg, src) {
    try { if (window.engine && window.engine.addLocalLog) window.engine.addLocalLog(msg, src); }
    catch(e) { console.log('[EE:'+src+']', msg); }
};

// Black Phillip
(function(){
    const h = new Date().getHours();
    if ((h === 0 || h === 23) && localStorage.getItem('ee_phillip_done') !== new Date().toDateString()) {
        localStorage.setItem('ee_phillip_done', new Date().toDateString());
        const logo = document.querySelector('img.brand-logo, img[src*="banner"]');
        if (logo) { logo.style.opacity = '0.3'; setTimeout(() => logo.style.opacity = '', 60000); }
        localStorage.setItem('ee_phillip_log', JSON.stringify({source:'BLACK PHILLIP', msg:'Wouldst thou like to live deliciously?', ts: Date.now()}));
    }
})();

// Malkovich
const _origSendOracle = window.sendOraclePrompt;
window.sendOraclePrompt = function() {
    const input = document.getElementById('oracle-prompt');
    if (input && input.value.trim().toLowerCase() === 'what is pastaay' && !localStorage.getItem('ee_malkovich')) {
        localStorage.setItem('ee_malkovich', '1');
        const chat = document.getElementById('oracle-chat');
        if (chat) chat.innerHTML += `<div class="mt-4 border-l-2 border-amber-500 pl-3 font-mono"><span class="text-amber-500 text-[9px] block mb-1 uppercase font-bold">MALKOVICH</span><span class="text-amber-300">Pastaay? Pastaay Pastaay! Pastaay Pastaay Pastaay!</span></div>`;
    }
    if (_origSendOracle) _origSendOracle();
};

// shaun of the dead
window._ee_deployCache = null;
const _origDeploy = window.deployOracleConfig;
if (_origDeploy) {
    window.deployOracleConfig = function(btn) {
        window._ee_deployCache = window.oracleYamlCache;
        window._ee_shaun_flag = true;
        return _origDeploy(btn);
    };
}

// Genesis
(function(){
    const k = 'ee_genesis_first';
    const now = Date.now();
    if (!localStorage.getItem(k)) localStorage.setItem(k, now);
    const days = Math.floor((now - parseInt(localStorage.getItem(k))) / 86400000);
    if (days >= 7 && localStorage.getItem('ee_genesis_done') !== String(days)) {
        localStorage.setItem('ee_genesis_done', days);
        document.documentElement.style.filter = 'grayscale(1)';
        setTimeout(() => document.documentElement.style.filter = '', 60000);
        localStorage.setItem('ee_genesis_log', JSON.stringify({source:'GENESIS', msg:'And on the seventh day, even chaos rested.', ts: Date.now()}));
    }
})();

// Replay stored logs
['ee_phillip_log','ee_genesis_log'].forEach(k => {
    const raw = localStorage.getItem(k); if (!raw) return;
    const log = JSON.parse(raw);
    const now = Date.now();
    if (now - log.ts < 120000) {
        setTimeout(() => {
            window._eeLog(log.msg, log.source);
        }, Math.random() * 3000 + 1000);
    }
});


// Matrix
window._ee_lastDeployYAML = null;
const _origDeployOracle = window.deployOracleConfig;
if (_origDeployOracle) {
    window.deployOracleConfig = function(btn) {
        const yaml = window.oracleYamlCache || '';
        if (yaml && yaml === window._ee_lastDeployYAML && !localStorage.getItem('ee_dejavu')) {
            localStorage.setItem('ee_dejavu', '1');
            document.body.insertAdjacentHTML('beforeend', '<span id=ee-cat style=position:fixed;bottom:10px;right:10px;font-size:40px;z-index:9999;opacity:0.8>🐈‍⬛</span>');
            setTimeout(() => { const c = document.getElementById('ee-cat'); if(c) c.remove(); }, 3000);
            setTimeout(() => window._eeLog('Déjà vu. A glitch in the Matrix.', 'TRINITY'), 800);
        }
        window._ee_lastDeployYAML = yaml;
        _origDeployOracle(btn);
    };
}

/* === END EASTER EGGS === */