// Apdex (Application Performance Index) Calculator
export class Apdex {
    constructor(thresholdMs = 400, sampleWindow = 200) {
        const t = Number(thresholdMs);
        this.T = Number.isFinite(t) && t > 0 ? t : 400;
        this.sampleWindow = Math.max(20, sampleWindow | 0);
        this.samples = [];
        this.reset();
    }

    record(elapsedMs, isError, meta = {}) {
        const ms = Number(elapsedMs);
        const safeMs = (Number.isFinite(ms) && ms >= 0) ? ms : 0;
        const error = Boolean(isError);

        this.total++;
        this.lastPing = safeMs;
        this.lastReason = meta.reason || (error ? 'error' : 'ok');
        this.lastStatus = meta.status || 0;

        if (meta.errorText) this.lastErrorText = meta.errorText;
        if (!error) this.lastErrorText = null;

        if (error) {
            this.frustrated++;
        } else {
            if (safeMs <= this.T) this.satisfied++;
            else if (safeMs <= this.T * 4) this.tolerating++;
            else this.frustrated++;
        }

        // Sliding window for P50/P95 calculations
        this.samples.push(safeMs);
        if (this.samples.length > this.sampleWindow) this.samples.shift();
    }

    getScore() {
        if (this.total === 0) return null; // Prevent NaN / false 100s
        return ((this.satisfied + (this.tolerating / 2)) / this.total) * 100.0;
    }

    getPercentiles() {
        if (this.samples.length === 0) return null;
        const sorted = [...this.samples].sort((a, b) => a - b);
        const pick = (p) => sorted[Math.min(sorted.length - 1, Math.floor((sorted.length - 1) * p))];
        return { p50: pick(0.5), p95: pick(0.95), p99: pick(0.99) };
    }

    getErrorRate() {
        return this.total > 0 ? (this.frustrated / this.total) * 100 : 0;
    }

    reset() {
        this.total = 0;
        this.satisfied = 0;
        this.tolerating = 0;
        this.frustrated = 0;
        this.lastPing = 0;
        this.lastReason = '';
        this.lastStatus = 0;
        this.lastErrorText = null;
        this.samples.length = 0;
    }
}

// Exponential Moving Average (EMA) Signal Smoother
export class EMAFilter {
    constructor(alpha = 0.2, initialValue = 100.0) {
        const a = Number(alpha);
        this.alpha = Number.isFinite(a) ? Math.min(1, Math.max(0, a)) : 0.2;
        const v = Number(initialValue);
        this.value = Number.isFinite(v) ? v : 0;
    }

    update(current) {
        const v = Number(current);
        if (!Number.isFinite(v)) return this.value;
        this.value = (this.value * (1 - this.alpha)) + (v * this.alpha);
        return this.value;
    }
}