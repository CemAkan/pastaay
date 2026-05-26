export const ChartManager = {
    semanticColors: { 'error': '#f87171', 'latency': '#fbbf24', 'drop': '#c084fc' },

    init: (id, metric) => {
        const chart = echarts.init(document.getElementById(id));
        let option = {};

        if (metric === 'global_rate') {
            option = {
                tooltip: { trigger: 'axis' },
                grid: { top: 20, bottom: 25, left: '6%', right: '4%', containLabel: true },
                xAxis: { type: 'category', data: Array(30).fill(''), show: false },
                yAxis: { type: 'value', splitLine: { lineStyle: { color: '#30363d', type: 'dashed' } }, axisLabel: { fontSize: 10, color: '#8b949e' } },
                series: [{ type: 'line', data: Array(30).fill(0), smooth: true, lineStyle: { color: '#56a4ff', width: 2.5 }, areaStyle: { color: 'rgba(86, 164, 255, 0.05)' }, symbol: 'none' }]
            };
        } else if (metric === 'impact_matrix') {
            option = {
                tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
                legend: { data: ['Error', 'Latency', 'Drop'], textStyle: { color: '#8b949e', fontSize: 10 }, bottom: 0 },
                grid: { top: 20, bottom: 40, left: '3%', right: '6%', containLabel: true },
                xAxis: { type: 'value', splitLine: { show: false }, axisLabel: { color: '#8b949e', fontSize: 10 } },
                yAxis: { type: 'category', data: [], axisLabel: { color: '#c9d1d9', fontSize: 10, interval: 0 } },
                series: [
                    { name: 'Error', type: 'bar', stack: 'total', itemStyle: { color: ChartManager.semanticColors['error'] }, data: [] },
                    { name: 'Latency', type: 'bar', stack: 'total', itemStyle: { color: ChartManager.semanticColors['latency'] }, data: [] },
                    { name: 'Drop', type: 'bar', stack: 'total', itemStyle: { color: ChartManager.semanticColors['drop'] }, data: [] }
                ]
            };
        }

        chart.setOption(option);
        return chart;
    },

    updateGlobalRate: (chart, history) => {
        chart.setOption({ series: [{ data: history }] });
    },

    updateImpactMatrix: (chart, targets, errorData, latData, dropData) => {
        let errArr = targets.map(t => errorData[t] || 0);
        let latArr = targets.map(t => latData[t] || 0);
        let dropArr = targets.map(t => dropData[t] || 0);

        chart.setOption({
            yAxis: { data: targets.map(t => {
                    let clean = t.includes(':') ? t.split(':').slice(1).join(':') : t;
                    return clean.length > 25 ? clean.substring(0,22)+"…" : clean;
                })},
            series: [ { data: errArr }, { data: latArr }, { data: dropArr } ]
        });
    }
};