package web

import (
	"html/template"
	"net/http"
)

// RegisterHandlers mounts the embedded Web Console routes to the Engine's mux.
func RegisterHandlers(mux *http.ServeMux) {

	tmpl := template.Must(template.ParseFS(TemplatesFS, "templates/*.html"))

	mux.Handle("/static/", http.FileServer(http.FS(StaticFS)))

	mux.HandleFunc("/console", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		
		data := map[string]interface{}{
			"Version": "v2.3.0",
		}

		if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
			http.Error(w, "Internal Engine Error: Template render failed", http.StatusInternalServerError)
		}
	})
}
