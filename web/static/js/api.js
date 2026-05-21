export const API = {
    // token reader
    getToken: () => document.getElementById('webhook-token')?.value || '',

    fetchMetrics: () => fetch('/console/api/metrics').then(r => r.ok ? r.json() : []),

    fetchState: function() {
        return fetch('/console/api/state', {
            headers: { 'X-Pastaay-Token': this.getToken() }
        }).then(r => r.ok ? r.json() : { policies: [], sensors_detail: {} });
    },

    probe: function(url) {
        return fetch('/console/api/probe', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'X-Pastaay-Token': this.getToken()
            },
            body: JSON.stringify({ url })
        }).then(r => r.json());
    },

    abortChaos: function() {
        return fetch('/console/api/rollback', {
            method: 'POST',
            headers: { 'X-Pastaay-Token': this.getToken() }
        }).then(r => {
            if(!r.ok) throw new Error("Rollback request failed");
            return r;
        });
    },

    lintPlan: function(yamlStr) {
        return fetch('/console/api/plan', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/yaml',
                'X-Pastaay-Token': this.getToken()
            },
            body: yamlStr
        }).then(r => r.ok ? r.json() : { status: "CRITICAL", score: 0, issues: ["Engine rejection: Validation endpoint failed or unauthorized."] });
    },

    deployChaos: (yamlStr, token) => fetch('/chaos/webhook', {
        method: 'POST',
        headers: { 'Content-Type': 'application/yaml', 'X-Pastaay-Token': token },
        body: yamlStr
    })
};