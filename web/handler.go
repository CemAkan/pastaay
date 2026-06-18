package web

import (
	"html/template"
	"net/http"

	"github.com/CemAkan/pastaay/pkg/config"
)

// RegisterHandlers mounts the embedded Web Console routes to the Engine's mux.
func RegisterHandlers(mux *http.ServeMux, mgr *config.Manager) {

	tmpl := template.Must(template.ParseFS(TemplatesFS, "templates/*.html"))

	mux.Handle("/static/", http.FileServer(http.FS(StaticFS)))

	mux.HandleFunc("/console", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tmpl.ExecuteTemplate(w, "layout.html", nil)
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

		tmpl.ExecuteTemplate(w, "dashboard_content", data)
	})
}
