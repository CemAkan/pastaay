import { UITemplates } from './ui_templates.js';
import { ChartManager } from './charts.js';

export class WidgetManager {
    constructor(engine) {
        this.engine = engine;
        this.list = [];
        this.counter = 0;

        window.addEventListener('resize', () => {
            this.list.forEach(w => {
                if (w.chart) w.chart.resize();
            });
        });
    }

    initSortable() {
        const grid = document.getElementById('dashboard-grid');
        if (grid && typeof Sortable !== 'undefined') {
            new Sortable(grid, {
                animation: 150,
                ghostClass: 'opacity-10',
                handle: '.drag-handle',
                onEnd: () => this.saveLayout()
            });
        }
    }

    saveLayout() {
        const grid = document.getElementById('dashboard-grid');
        if (!grid) return;
        const currentLayout = [];

        Array.from(grid.children).forEach(wrapper => {
            if (wrapper.id === 'empty-state') return;
            const wId = wrapper.getAttribute('data-widget-id');
            if (wId) {
                const w = this.list.find(x => x.id === wId);
                if (w) currentLayout.push({metric: w.metric, expanded: w.expanded});
            }
        });

        localStorage.setItem('pastaay_layout', JSON.stringify(currentLayout));
    }

    loadLayout() {
        try {
            const savedRaw = localStorage.getItem('pastaay_layout');
            if (savedRaw !== null) {
                const saved = JSON.parse(savedRaw);
                if (Array.isArray(saved) && saved.length > 0) {
                    saved.forEach(w => this.add(w.metric, w.expanded));
                    return;
                }
            }
        } catch (e) {}

        this.add('global_rate', true);
        this.add('log', false);
    }

    toggleExpand(id) {
        const wrapper = document.querySelector(`[data-widget-id="${id}"]`);
        const details = document.getElementById(`${id}-details`);
        const btn = document.getElementById(`${id}-expand-btn`);
        const w = this.list.find(x => x.id === id);
        if (!wrapper || !details || !w) return;

        w.expanded = !w.expanded;

        // Inception
        if (!window._eeExpandStack) window._eeExpandStack = [];
        const nowI = Date.now();
        window._eeExpandStack = window._eeExpandStack.filter(e => nowI - e.ts < 10000);
        if (!window._eeExpandStack.find(e => e.id === id)) {
            window._eeExpandStack.push({id, ts: nowI});
        }
        if (window._eeExpandStack.length >= 3 && !localStorage.getItem("ee_inception")) {
            localStorage.setItem("ee_inception", "1");
            const grid = document.getElementById("dashboard-grid");
            if (grid) { grid.style.transition = "transform 1s"; grid.style.transform = "rotate(2deg)"; setTimeout(() => { grid.style.transform = ""; }, 1000); }
            setTimeout(() => window._eeLog("We need to go deeper.", "COBB"), 500);
        }


        if (w.expanded) {
            wrapper.classList.remove('col-span-1');
            wrapper.classList.add('col-span-2');
            details.classList.add('open');
            if (btn) btn.textContent = "SHRINK";
        } else {
            wrapper.classList.remove('col-span-2', 'col-span-full');
            wrapper.classList.add('col-span-1');
            details.classList.remove('open');
            if (btn) btn.textContent = "EXPAND";
        }

        setTimeout(() => { if (w.chart) w.chart.resize(); }, 50);
        this.saveLayout();
    }

    add(metric, expanded = false) {
        try {
            if (this.list.find(w => w.metric === metric)) {
                this.engine.addLocalLog(`SYS ┠ Guard Block: Module [${metric}] is already mounted.`, 'system');
                return;
            }

            const emptyState = document.getElementById('empty-state');
            if (emptyState) emptyState.classList.add('hidden');

            const id = 'widget-' + this.counter++;
            let titleStr = metric.replace('_', ' ').toUpperCase();
            const detailState = expanded ? 'open' : 'closed';

            let contentHTML = (metric === 'pinger') ? UITemplates.getPingerHTML(id, detailState) :
                (metric === 'log') ? UITemplates.getLogHTML(id, detailState) :
                    UITemplates.getChartHTML(id, detailState);

            const expandText = expanded ? 'SHRINK' : 'EXPAND';
            const sizeClass = expanded ? 'col-span-2' : 'col-span-1';

            const wrapper = document.createElement('div');
            wrapper.setAttribute('data-widget-id', id);
            wrapper.className = `widget-wrapper ${sizeClass}`;

            wrapper.innerHTML = UITemplates.getWrapper(id, metric, titleStr, expandText, contentHTML, "");
            document.getElementById('dashboard-grid').appendChild(wrapper);

            let chart = null;
            if (metric !== 'log' && metric !== 'pinger') {
                try {
                    chart = ChartManager.init(id, metric);
                    setTimeout(() => { if (chart) chart.resize(); }, 150);
                } catch (e) { console.error("Chart Render Failed, continuing...", e); }
            }

            this.list.push({
                id, chart, metric, type: metric, expanded,
                history: metric === 'pinger' ? Array(30).fill(100) : Array(30).fill(0)
            });

            if (metric === 'log') {
                this.engine.rebuildLogFilters();
                this.engine.updateLogWidget();
            }
            this.saveLayout();
        } catch(err) {
            console.error("Widget render failed, isolating error:", err);
        }
    }

    remove(id) {
        const w = this.list.find(x => x.id === id);
        if (!w) return;

        try { if (w.chart && typeof w.chart.destroy === 'function') w.chart.destroy(); } catch (e) {}
        try { if (w.metric === 'pinger' && this.engine && this.engine.pinger) this.engine.pinger.stop(); } catch (e) {}

        const wrapper = document.querySelector(`[data-widget-id="${id}"]`);
        if (wrapper && wrapper.parentElement) wrapper.parentElement.removeChild(wrapper);

        this.list = this.list.filter(x => x.id !== id);
        this.saveLayout();

        // Bagel
        if (this.list.length === 0 && !localStorage.getItem('ee_bagel')) {
            localStorage.setItem('ee_bagel', '1');
            const g = document.getElementById('dashboard-grid');
            if (g) { g.style.position = 'relative'; g.insertAdjacentHTML('beforeend', '<span id=ee-bagel style=position:absolute;top:50%;left:50%;transform:translate(-50%,-50%);font-size:80px;z-index:999;animation:eeSpin 1s linear infinite>🥯</span>'); setTimeout(() => { const b = document.getElementById('ee-bagel'); if(b) b.remove(); }, 3000); }
            if (window._eeLog) window._eeLog('Nothing matters.', 'JOY');
        }

        if (this.list.length === 0) {
            const emptyState = document.getElementById('empty-state');
            if (emptyState) emptyState.classList.remove('hidden');
        }
    }

}