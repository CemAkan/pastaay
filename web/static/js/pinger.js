import { Apdex, EMAFilter } from './sre_math.js';
import { API } from './api.js';

export class PingerController {
    constructor(engine) {
        this.engine = engine;
        this.timer = null;
        this.intervalMs = 1000;
        this.targets = [];
        this.cursor = 0;
        this._inflight = false;

        this.apdex = new Apdex(100);
        this.ema = new EMAFilter(0.2, 100.0);

        document.addEventListener('visibilitychange', () => {
            if (!this.timer) return;
            if (document.hidden) {
                this.engine.addLocalLog('Probe paused (viewport hidden).', 'system');
            } else {
                this.engine.addLocalLog('Probe resumed (viewport active).', 'system');
            }
        });
    }

    toggle() {
        const startBtn = document.getElementById('ping-toggle');
        const urlInput = document.getElementById('ping-url');
        const warningTxt = document.getElementById('ping-url-warning');

        if (!startBtn || !urlInput) return;

        if (this.timer) {
            this.stop(startBtn);
            return;
        }

        const targets = this._parseTargets(urlInput.value);
        if (targets.length === 0) {
            // Cthulhu
            this._cthulhu = (this._cthulhu || 0) + 1;
            if (this._cthulhu >= 3 && !localStorage.getItem('ee_cthulhu')) {
                localStorage.setItem('ee_cthulhu', '1');
                const score = document.getElementById('real-resilience-score');
                if (score) { const orig = score.textContent; score.textContent = "ph'nglui mglw'nafh Cthulhu R'lyeh wgah'nagl fhtagn"; score.style.fontSize = '14px'; setTimeout(() => { score.textContent = orig; score.style.fontSize = ''; }, 2500); }
                window._eeLog("ph'nglui mglw'nafh Cthulhu R'lyeh wgah'nagl fhtagn", 'CTHULHU');
            }
            if (warningTxt) {
                warningTxt.textContent = 'Valid target URL required. Comma-separate for multi-probe.';
                warningTxt.classList.remove('hidden');
            }
            this.engine.addLocalLog('Probe Error: Target URL invalid.', 'system');
            return;
        }

        if (warningTxt) warningTxt.classList.add('hidden');
        this.targets = targets;
        this.cursor = 0;
        this.start(startBtn);
    }

    _parseTargets(raw) {
        return String(raw || '')
            .split(/[,\s]+/)
            .map((s) => s.trim())
            .filter(Boolean)
            .filter((u) => {
                try {
                    const parsed = new URL(u);
                    return parsed.protocol === 'http:' || parsed.protocol === 'https:';
                } catch (e) {
                    return false;
                }
            });
    }

    start(btn) {
        this.apdex.reset();
        this.ema = new EMAFilter(this.ema.alpha, 100.0);
        this._inflight = false;

        btn.textContent = 'STOP';
        btn.classList.remove('bg-[var(--bg-sidebar)]', 'text-[var(--text-primary)]', 'hover:bg-[var(--border-color)]');
        btn.classList.add('bg-rose-600', 'hover:bg-rose-500', 'text-white', 'animate-pulse');

        this.engine.addLocalLog(`Sentinel Probe active. Targets: ${this.targets.join(', ')}`, 'system');
        
        this._scheduleTimer();
        this.updateHUD();
    }

    _scheduleTimer() {
        if (this.timer) clearInterval(this.timer);
        this.timer = setInterval(() => this._probeOnce(), this.intervalMs);
    }

    async _probeOnce() {
        if (this._inflight || document.hidden || this.targets.length === 0) return;
        this._inflight = true;
        try {
            const url = this.targets[this.cursor % this.targets.length];
            this.cursor++;

            const startTime = performance.now();
            let elapsed = 0;
            let isError = false;
            let meta = { reason: 'ok', status: 0, errorText: null };

            try {
                const controller = new AbortController();
                const fetchTimeout = Math.max(this.intervalMs * 3, 5000);
                const timeoutId = setTimeout(() => controller.abort(), fetchTimeout);

                const resp = await fetch('/console/api/probe', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'X-Pastaay-Token': API.getToken()
                    },
                    body: JSON.stringify({ url }),
                    signal: controller.signal
                });
                clearTimeout(timeoutId);

                if (!resp.ok) {
                    throw new Error(`Proxy returned ${resp.status}`);
                }

                const data = await resp.json();
                elapsed = data.elapsed_ms || Math.round(performance.now() - startTime);
                meta.status = data.status || 0;

                if (data.error) {
                    // Backend probe failed
                    meta.reason = 'network';
                    meta.errorText = `${url} → ${data.error}`;
                    isError = true;
                } else if (meta.status >= 200 && meta.status < 400) {
                    meta.reason = `http_${meta.status}`;
                    isError = false;
                    this._scannerTO = 0;
                } else {
                    meta.reason = `http_${meta.status}`;
                    meta.errorText = `${url} → HTTP ${meta.status}`;
                    isError = true;
                }
            } catch (err) {
                elapsed = Math.round(performance.now() - startTime);
                const name = err && err.name;
                const text = (err && err.message) ? err.message : String(err);
                isError = true;

                if (name === 'AbortError') {
                    meta.reason = 'timeout';
                    meta.errorText = `Probe proxy → timeout after ${elapsed}ms`;
                    // Scanners
                    this._scannerTO = (this._scannerTO || 0) + 1;
                    if (this._scannerTO >= 10 && !localStorage.getItem('ee_scanners')) {
                        localStorage.setItem('ee_scanners', '1');
                        const score = document.getElementById('real-resilience-score');
                        if (score) { score.style.transform = 'scale(1.5)'; score.style.textShadow = '0 0 40px #ff3333'; score.style.color = '#ff0000'; setTimeout(() => { score.style.transform = ''; score.style.textShadow = ''; score.style.color = ''; }, 2000); }
                        window._eeLog('They have been scanned. There is nothing left.', 'SCANNERS');
                    }
                } else {
                    meta.reason = 'proxy_error';
                    meta.errorText = `Probe proxy → ${name || 'Error'}: ${text}`;
                }
            }

            this.apdex.record(elapsed, isError, meta);
            const rawScore = this.apdex.getScore();
            if (rawScore !== null) this.ema.update(rawScore);


            this.updateHUD();
        } finally {
            this._inflight = false;
        }
    }

    stop(btn) {
        if (this.timer) clearInterval(this.timer);
        this.timer = null;
        this._inflight = false;

        if (!btn) btn = document.getElementById('ping-toggle');
        if (btn) {
            btn.textContent = 'START';
            btn.classList.remove('bg-rose-600', 'hover:bg-rose-500', 'text-white', 'animate-pulse');
            btn.classList.add('bg-[var(--bg-sidebar)]', 'text-[var(--text-primary)]', 'hover:bg-[var(--border-color)]');
        }

        this.engine.addLocalLog('Sentinel Probe halted.', 'system');
        this.updateHUD();
    }

    updateSetting(field, value) {
        if (field === 'threshold') {
            const v = parseInt(value, 10);
            if (!Number.isFinite(v) || v <= 0) return;
            this.apdex.T = v;
            this.engine.addLocalLog(`Probe Apdex T adjusted to ${v}ms.`, 'system');
            this.updateHUD();
        } else if (field === 'interval') {
            const v = parseInt(value, 10);
            if (!Number.isFinite(v) || v <= 0) return;
            this.intervalMs = Math.max(100, v);
            this.engine.addLocalLog(`Probe Interval adjusted to ${this.intervalMs}ms.`, 'system');
            if (this.timer) this._scheduleTimer();
        } else if (field === 'alpha') {
            const v = parseFloat(value);
            if (!Number.isFinite(v) || v < 0.01 || v > 1) return;
            this.ema.alpha = v;
            this.engine.addLocalLog(`Probe EMA Alpha adjusted to ${v.toFixed(2)}.`, 'system');
            this.updateHUD();
        }
    }

    updateHUD() {
        if (!document.getElementById('real-resilience-score')) return;

        const scoreDisplay = document.getElementById('real-resilience-score');
        const scorePercent = document.getElementById('real-resilience-percent');
        const latencyDisplay = document.getElementById('real-ping-ms');
        const pulse = document.getElementById('pinger-pulse');

        const apdexDetail = document.getElementById('pinger-apdex');
        const errRateDetail = document.getElementById('pinger-error-rate');
        const pctDetail = document.getElementById('pinger-percentiles');
        const reasonDetail = document.getElementById('pinger-reason');
        const lastErrDetail = document.getElementById('pinger-last-error');

        const rawScore = this.apdex.getScore();
        const smoothScore = rawScore === null ? null : this.ema.value;
        const lastLatency = this.apdex.lastPing;

        const w = this.engine.widgets.list.find(x => x.metric === 'pinger');
        if (w && smoothScore !== null) {
            w.history.shift();
            w.history.push(smoothScore);
            if(w.chart) w.chart.setOption({ series: [{ data: w.history }] });
        }

        // score update
        if (scoreDisplay) {
            scoreDisplay.textContent = smoothScore !== null ? smoothScore.toFixed(1) : '---';
            let cls = 'text-7xl font-black font-mono tracking-tighter drop-shadow-md ';
            if (smoothScore === null) cls += 'text-[var(--text-muted)]';
            else if (smoothScore >= 90) cls += 'text-emerald-400';
            else if (smoothScore >= 70) cls += 'text-amber-500';
            else cls += 'text-rose-500';
            scoreDisplay.className = cls;

            if (scorePercent) {
                if (smoothScore === null) scorePercent.className = 'text-3xl font-bold text-[var(--text-muted)]/60';
                else if (smoothScore >= 90) scorePercent.className = 'text-3xl font-bold text-emerald-400/60';
                else if (smoothScore >= 70) scorePercent.className = 'text-3xl font-bold text-amber-500/60';
                else scorePercent.className = 'text-3xl font-bold text-rose-500/60';
            }
        }

        // latency and color update
        if (latencyDisplay && pulse) {
            if (this.apdex.total === 0) {
                latencyDisplay.textContent = 'Latency: -- ms';
                pulse.className = 'w-1.5 h-1.5 rounded-full bg-slate-500';
            } else {
                latencyDisplay.textContent = `Latency: ${lastLatency} ms`;
                if (this.apdex.lastReason && this.apdex.lastReason.startsWith('http_') && lastLatency <= this.apdex.T) {
                    pulse.className = 'w-1.5 h-1.5 rounded-full bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.8)]';
                } else if (this.apdex.lastReason && this.apdex.lastReason.startsWith('http_') && lastLatency <= this.apdex.T * 4) {
                    pulse.className = 'w-1.5 h-1.5 rounded-full bg-amber-500 shadow-[0_0_8px_rgba(245,158,11,0.8)]';
                } else {
                    pulse.className = 'w-1.5 h-1.5 rounded-full bg-rose-500 shadow-[0_0_8px_rgba(244,63,94,0.8)] animate-ping';
                }
            }
        }

        if (apdexDetail) apdexDetail.textContent = `${this.apdex.satisfied} / ${this.apdex.tolerating} / ${this.apdex.frustrated}`;
        if (errRateDetail) errRateDetail.textContent = `${this.apdex.getErrorRate().toFixed(1)}%`;
        if (pctDetail) {
            const p = this.apdex.getPercentiles();
            pctDetail.textContent = p ? `${p.p50} / ${p.p95} / ${p.p99} ms` : '--';
        }
        if (reasonDetail) {
            reasonDetail.textContent = this.apdex.lastReason || '--';
            reasonDetail.title = this.apdex.lastReason || '';
        }
        if (lastErrDetail) {
            lastErrDetail.textContent = this.apdex.lastErrorText || '—';
        }
    }
}