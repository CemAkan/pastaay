package web

import (
	"bufio"
	"context"
	"crypto/subtle"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/CemAkan/pastaay/docs"
	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/CemAkan/pastaay/pkg/guard"
	"github.com/CemAkan/pastaay/pkg/oracle"
	"github.com/CemAkan/pastaay/pkg/telemetry"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func emitLocal(name, msg string) { telemetry.Emit("local", name, msg) }

func startKubeLogStreamer(ctx context.Context, clientset *kubernetes.Clientset, namespace, podName string) {
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{Follow: true})
	stream, err := req.Stream(ctx)
	if err != nil {
		log.Printf("[Pastaay-K8s] stream fail: %s", podName)
		return
	}
	defer stream.Close()

	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		telemetry.Emit("kube", podName, scanner.Text())
	}
}

func EmitLog(source, name, msg string) {
	telemetry.Emit(source, name, msg)
}

// watchAndStreamPods uses K8s Watch API
func watchAndStreamPods(ctx context.Context, clientset *kubernetes.Clientset, namespace string) {
	activeStreams := make(map[string]context.CancelFunc)
	var mu sync.Mutex

	backoff := time.Second
	for {
		if ctx.Err() != nil {
			mu.Lock()
			for _, cancel := range activeStreams {
				cancel()
			}
			mu.Unlock()
			return
		}

		w, err := clientset.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{})
		if err != nil {
			log.Printf("[Pastaay-K8s] watch failed: %v (retry %v)", err, backoff)
			emitLocal("kube-watcher", "watch failed: "+err.Error())
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = time.Second
		emitLocal("kube-watcher", "watch established on namespace "+namespace)

		for event := range w.ResultChan() {
			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				continue
			}
			mu.Lock()
			switch event.Type {
			case watch.Added, watch.Modified:
				if pod.Status.Phase == corev1.PodRunning {
					if _, exists := activeStreams[pod.Name]; !exists {
						streamCtx, cancel := context.WithCancel(ctx)
						activeStreams[pod.Name] = cancel
						emitLocal("kube-watcher", "attaching stream to pod "+pod.Name)
						go startKubeLogStreamer(streamCtx, clientset, namespace, pod.Name)
					}
				}
			case watch.Deleted:
				if cancel, exists := activeStreams[pod.Name]; exists {
					cancel()
					delete(activeStreams, pod.Name)
					emitLocal("kube-watcher", "detached stream from pod "+pod.Name)
				}
			}
			mu.Unlock()
		}
		log.Printf("[Pastaay-K8s] watch channel closed, reconnecting")
		emitLocal("kube-watcher", "watch channel closed, reconnecting")
	}
}

// Auth middleware
func requireConsoleToken(expected string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if expected == "" {
			next(w, r)
			return
		}
		got := r.Header.Get("X-Pastaay-Token")
		if subtle.ConstantTimeCompare([]byte(got), []byte(expected)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func renderHTML(tmpl *template.Template, w http.ResponseWriter, page string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout.html", map[string]string{"Page": page}); err != nil {
		log.Printf("[Pastaay-Web] template render error (%s): %v", page, err)
	}
}

// Handlers

func handleOracle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Provider, Key, Model, Intensity, Prompt, Context string
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	resp, err := oracle.AskOracle(req.Provider, req.Key, req.Model, req.Intensity, req.Prompt, req.Context)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"response": resp, "yaml": oracle.ExtractYAML(resp)})
}

func handlePlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()
	var cfg config.PastaayConfig
	if err := yaml.NewDecoder(r.Body).Decode(&cfg); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	json.NewEncoder(w).Encode(guard.Analyze(&cfg))
}

func handleRollback(mgr *config.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mgr.Update(&config.PastaayConfig{Version: 1, Policies: []config.Policy{}})
		emitLocal("pastaay", "rollback executed — all policies cleared")
		w.Write([]byte(`{"status":"ok"}`))
	}
}

func handleState(mgr *config.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		cfg := mgr.GetRawConfig()

		sensors := mgr.GetSensorStatuses()
		sensorCount := 0
		for _, s := range sensors {
			if s == "healthy" || s == "connected" {
				sensorCount++
			}
		}

		activePolicies := 0
		var policies []config.Policy = []config.Policy{}

		if cfg != nil {
			activePolicies = len(cfg.Policies)
			policies = cfg.Policies
		}

		rawYAML := "version: 1\npolicies: []"
		if cfg != nil {
			if b, err := yaml.Marshal(cfg); err == nil {
				rawYAML = string(b)
			} else {
				log.Printf("[Pastaay-Web] YAML marshal error: %v", err)
			}
		}

		response := map[string]interface{}{
			"active_policies": activePolicies,
			"active_sensors":  sensorCount,
			"sensors_detail":  sensors,
			"raw_yaml":        rawYAML,
			"policies":        policies,
			"engine_logs":     telemetry.Snapshot(),
		}

		json.NewEncoder(w).Encode(response)
	}
}

func handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	type metricData struct {
		Target string `json:"target"`
		Type   string `json:"type"`
		Value  int    `json:"value"`
	}

	metricsList := []metricData{}
	for _, mf := range mfs {
		if mf.GetName() == "pastaay_injected_faults_total" {
			for _, m := range mf.GetMetric() {
				var target, fType string
				for _, lp := range m.GetLabel() {
					if lp.GetName() == "target" {
						target = lp.GetValue()
					}
					if lp.GetName() == "fault_type" {
						fType = lp.GetValue()
					}
				}
				metricsList = append(metricsList, metricData{Target: target, Type: fType, Value: int(m.GetCounter().GetValue())})
			}
		}
	}
	json.NewEncoder(w).Encode(metricsList)
}

func handleProbe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if req.URL == "" {
		http.Error(w, "url required", http.StatusBadRequest)
		return
	}

	start := time.Now()
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(req.URL)
	elapsed := time.Since(start).Milliseconds()

	result := map[string]interface{}{
		"elapsed_ms": elapsed,
	}

	if err != nil {
		result["status"] = 0
		result["error"] = err.Error()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
		return
	}
	defer resp.Body.Close()

	result["status"] = resp.StatusCode
	result["error"] = nil

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func handleDiscover(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	targets := make(map[string]bool)
	targetList := []string{}
	for _, mf := range mfs {
		if mf.GetName() == "pastaay_injected_faults_total" {
			for _, m := range mf.GetMetric() {
				for _, lp := range m.GetLabel() {
					if lp.GetName() == "target" {
						t := lp.GetValue()
						if !targets[t] {
							targets[t] = true
							targetList = append(targetList, t)
						}
					}
				}
			}
		}
	}
	json.NewEncoder(w).Encode(targetList)
}

func RegisterHandlers(mux *http.ServeMux, mgr *config.Manager) {
	adminToken := os.Getenv("PASTAAY_WEBHOOK_TOKEN")

	// K8s watcher
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		if restConfig, err := rest.InClusterConfig(); err == nil {
			if clientset, csErr := kubernetes.NewForConfig(restConfig); csErr == nil {
				go watchAndStreamPods(context.Background(), clientset, getEnv("PASTAAY_K8S_NAMESPACE", "default"))
			}
		}
	}

	tmpl := template.Must(template.ParseFS(TemplatesFS, "templates/*.html"))
	mux.Handle("/static/", http.FileServer(http.FS(StaticFS)))
	mux.Handle("/console/docs/raw/", http.StripPrefix("/console/docs/raw/", http.FileServer(http.FS(docs.FS))))

	mux.HandleFunc("/console", func(w http.ResponseWriter, r *http.Request) { renderHTML(tmpl, w, "dashboard") })
	mux.HandleFunc("/console/docs", func(w http.ResponseWriter, r *http.Request) { renderHTML(tmpl, w, "docs") })
	mux.HandleFunc("/console/builder", func(w http.ResponseWriter, r *http.Request) { renderHTML(tmpl, w, "builder") })

	mux.HandleFunc("/console/api/metrics", handleMetrics)
	mux.HandleFunc("/console/api/state", requireConsoleToken(adminToken, handleState(mgr)))
	mux.HandleFunc("/console/api/probe", requireConsoleToken(adminToken, handleProbe))
	mux.HandleFunc("/console/api/discover", requireConsoleToken(adminToken, handleDiscover))
	mux.HandleFunc("/console/api/oracle", requireConsoleToken(adminToken, handleOracle))
	mux.HandleFunc("/console/api/plan", requireConsoleToken(adminToken, handlePlan))
	mux.HandleFunc("/console/api/rollback", requireConsoleToken(adminToken, handleRollback(mgr)))
}

func getEnv(k, f string) string {
	if v, ok := os.LookupEnv(k); ok {
		return v
	}
	return f
}
