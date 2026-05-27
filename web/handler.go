package web

import (
	"bufio"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
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

const (
	maxAPIRequestBytes = 1 << 20 // 1 MiB
	probeTimeout       = 5 * time.Second
)

var (
	modelNameRe  = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)
	allowedProvs = map[string]bool{"openai": true, "deepseek": true, "gemini": true, "anthropic": true}
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

func EmitLog(source, name, msg string) { telemetry.Emit(source, name, msg) }

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

// requireConsoleToken enforces auth; in the empty-token mode it logs every
// request to ensure the security gap is loud.
func requireConsoleToken(expected string, next http.HandlerFunc) http.HandlerFunc {
	disabled := expected == ""
	return func(w http.ResponseWriter, r *http.Request) {
		if disabled {
			log.Printf("[Pastaay-Console] SECURITY: unauthenticated %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
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

// handleOracle delegates to the configured LLM provider, validates the modeln// name against a regex allow‑list, and returns the generated YAML blueprint.

func handleOracle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Provider, Key, Model, Intensity, Prompt, Context string
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxAPIRequestBytes)).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if !allowedProvs[req.Provider] {
		http.Error(w, "unsupported provider", http.StatusBadRequest)
		return
	}
	if req.Model != "" && !modelNameRe.MatchString(req.Model) {
		http.Error(w, "invalid model name", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	resp, err := oracle.AskOracle(req.Provider, req.Key, req.Model, req.Intensity, req.Prompt, req.Context)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"response": resp, "yaml": oracle.ExtractYAML(resp)})
}

func handlePlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()

	result, err := guard.PlanFromRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func handleRollback(mgr *config.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		mgr.Update(&config.PastaayConfig{Version: 1, Policies: []config.Policy{}})
		emitLocal("pastaay", "rollback executed — all policies cleared")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
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

		_ = json.NewEncoder(w).Encode(response)
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
	_ = json.NewEncoder(w).Encode(metricsList)
}

// safeProbeTransport rejects connections to private / loopback / link-local /
// multicast / unspecified ranges and refuses any redirect.
var safeProbeTransport = &http.Transport{
	DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		ips, err := (&net.Resolver{}).LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if isInternalIP(ip.IP) {
				return nil, fmt.Errorf("pastaay: refused to dial internal address %s", ip.IP)
			}
		}
		d := &net.Dialer{Timeout: 3 * time.Second}
		return d.DialContext(ctx, network, addr)
	},
	ResponseHeaderTimeout: probeTimeout,
}

var safeProbeClient = &http.Client{
	Transport: safeProbeTransport,
	Timeout:   probeTimeout,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return errors.New("pastaay: redirects are disabled for probe targets")
	},
}

func isInternalIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	// IPv4: AWS/GCP/Azure metadata (169.254.169.254 caught by IsLinkLocal),
	// shared address space (100.64.0.0/10).
	if v4 := ip.To4(); v4 != nil {
		if v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
			return true
		}
	}
	return false
}

// handleProbe performs an HTTP GET against a user‑supplied URL through an// hardened transport that refuses connections to internal/loopback addresses.

func handleProbe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxAPIRequestBytes)).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if req.URL == "" {
		http.Error(w, "url required", http.StatusBadRequest)
		return
	}

	u, err := url.Parse(req.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		http.Error(w, "url must be http(s) with explicit host", http.StatusBadRequest)
		return
	}

	result := map[string]interface{}{}
	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), probeTimeout)
	defer cancel()

	probeReq, err := http.NewRequestWithContext(ctx, http.MethodGet, req.URL, nil)
	if err != nil {
		result["status"] = 0
		result["error"] = err.Error()
		_ = json.NewEncoder(w).Encode(result)
		return
	}

	start := time.Now()
	resp, err := safeProbeClient.Do(probeReq)
	elapsed := time.Since(start).Milliseconds()
	result["elapsed_ms"] = elapsed

	if err != nil {
		result["status"] = 0
		result["error"] = err.Error()
		_ = json.NewEncoder(w).Encode(result)
		return
	}
	defer resp.Body.Close()
	// Drain at most 4 KiB so we never load attacker bodies.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	result["status"] = resp.StatusCode
	result["error"] = nil
	_ = json.NewEncoder(w).Encode(result)
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
	_ = json.NewEncoder(w).Encode(targetList)
}

func RegisterHandlers(mux *http.ServeMux, mgr *config.Manager) {
	adminToken := os.Getenv("PASTAAY_WEBHOOK_TOKEN")

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
