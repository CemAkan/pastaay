package web

import (
	"encoding/json"
	"html/template"
	"net/http"

	"github.com/CemAkan/pastaay/docs"
	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/prometheus/client_golang/prometheus"
)

// RegisterHandlers mounts the embedded Web Console routes to the Engine's mux.
func RegisterHandlers(mux *http.ServeMux, mgr *config.Manager) {

	tmpl := template.Must(template.ParseFS(TemplatesFS, "templates/*.html"))

	mux.Handle("/static/", http.FileServer(http.FS(StaticFS)))

	mux.Handle("/console/docs/raw/", http.StripPrefix("/console/docs/raw/", http.FileServer(http.FS(docs.FS))))

	mux.HandleFunc("/console", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "layout.html", map[string]string{"Page": "dashboard"}); err != nil {
			http.Error(w, "Template Render Error: "+err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/console/docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "layout.html", map[string]string{"Page": "docs"}); err != nil {
			http.Error(w, "Template Render Error: "+err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/console/builder", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "layout.html", map[string]string{"Page": "builder"}); err != nil {
			http.Error(w, "Template Render Error: "+err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/console/api/dashboard", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		cfg := mgr.GetRawConfig()
		activeCount := 0
		var policies []config.Policy
		if cfg != nil {
			activeCount = len(cfg.Policies)
			policies = cfg.Policies
		}

		sensorCount := 0
		for _, status := range mgr.GetSensorStatuses() {
			if status == "healthy" || status == "connected" || status == "initializing" {
				sensorCount++
			}
		}

		data := map[string]interface{}{
			"ActiveCount": activeCount,
			"SensorCount": sensorCount,
			"Policies":    policies,
		}

		if err := tmpl.ExecuteTemplate(w, "dashboard_content", data); err != nil {
			http.Error(w, "Dashboard Component Render Error: "+err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/console/api/chart", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		mfs, err := prometheus.DefaultGatherer.Gather()
		if err != nil {
			http.Error(w, "{}", http.StatusInternalServerError)
			return
		}

		labels := []string{}
		values := []int{}

		for _, mf := range mfs {
			if mf.GetName() == "pastaay_injected_faults_total" {
				for _, m := range mf.GetMetric() {
					var target string
					for _, lp := range m.GetLabel() {
						if lp.GetName() == "target" {
							target = lp.GetValue()
						}
					}
					labels = append(labels, target)
					values = append(values, int(m.GetCounter().GetValue()))
				}
			}
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"labels": labels,
			"values": values,
		})
	})
}
