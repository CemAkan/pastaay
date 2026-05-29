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
	"strings"
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
	probeMaxBodyBytes  = 4096
)

var (
	modelNameRe  = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)
	allowedProvs = map[string]bool{"openai": true, "deepseek": true, "gemini": true, "anthropic": true}
)

// providerKey returns the API key for the given provider from env.
func providerKey(provider string) string {
	switch strings.ToLower(provider) {
	case "openai":
		return os.Getenv("PASTAAY_OPENAI_KEY")
	case "deepseek":
		return os.Getenv("PASTAAY_DEEPSEEK_KEY")
	case "gemini":
		return os.Getenv("PASTAAY_GEMINI_KEY")
	case "anthropic":
		return os.Getenv("PASTAAY_ANTHROPIC_KEY")
	}
	return ""
}

func emitLocal(name, msg string) { telemetry.Emit("local", name, msg) }

// kubeLogScannerMaxBytes is the max line length for K8s log streams (1 MiB).
const kubeLogScannerMaxBytes = 1 << 20

func startKubeLogStreamer(ctx context.Context, clientset *kubernetes.Clientset, namespace, podName string) {
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{Follow: true})
	stream, err := req.Stream(ctx)
	if err != nil {
		log.Printf("[Pastaay-K8s] stream fail: %s", podName)
		return
	}
	defer stream.Close()

	scanner := bufio.NewScanner(stream)
	scanner.Buffer(make([]byte, 0, 64*1024), kubeLogScannerMaxBytes)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		telemetry.Emit("kube", podName, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Printf("[Pastaay-K8s] scanner closed for %s: %v", podName, err)
	}
}

func EmitLog(source, name, msg string) { telemetry.Emit(source, name, msg) }

func watchAndStreamPods(ctx context.Context, clientset *kubernetes.Clientset, namespace string) {
	activeStreams := make(map[string]context.CancelFunc)
	var mu sync.Mutex

	stopAll := func() {
		mu.Lock()
		for _, cancel := range activeStreams {
			cancel()
		}
		activeStreams = make(map[string]context.CancelFunc)
		mu.Unlock()
	}

	backoff := time.Second
	for {
		if ctx.Err() != nil {
			stopAll()
			return
		}

		w, err := clientset.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{})
		if err != nil {
			log.Printf("[Pastaay-K8s] watch failed: %v (retry %v)", err, backoff)
			emitLocal("kube-watcher", "watch failed: "+err.Error())
			select {
			case <-ctx.Done():
				stopAll()
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
				// Call w.Stop outside the mutex to avoid deadlocking client-go.
				if event.Type == watch.Error {
					w.Stop()
				}
				continue
			}
			switch event.Type {
			case watch.Added, watch.Modified:
				if pod.Status.Phase == corev1.PodRunning {
					mu.Lock()
					if _, exists := activeStreams[pod.Name]; !exists {
						streamCtx, cancel := context.WithCancel(ctx)
						activeStreams[pod.Name] = cancel
						emitLocal("kube-watcher", "attaching stream to pod "+pod.Name)
						go startKubeLogStreamer(streamCtx, clientset, namespace, pod.Name)
					}
					mu.Unlock()
				}
			case watch.Deleted:
				mu.Lock()
				if cancel, exists := activeStreams[pod.Name]; exists {
					cancel()
					delete(activeStreams, pod.Name)
					emitLocal("kube-watcher", "detached stream from pod "+pod.Name)
				}
				mu.Unlock()
			case watch.Error:
				w.Stop()
			}
		}
		// When the watch channel closes (network blip, expired RV, etcd
		// hiccup), we MUST tear down per-pod streams. Otherwise streamers
		// keep tailing logs against the *old* connection's resource handles
		// — and any pod that was deleted while the watch was down would
		// never receive its cancel(). This loop reconnects with a fresh
		// resource version and rebuilds streams via "Added" events.
		stopAll()
		log.Printf("[Pastaay-K8s] watch channel closed, reconnecting")
		emitLocal("kube-watcher", "watch channel closed, reconnecting")
	}
}

// requireConsoleToken enforces auth.
func requireConsoleToken(expected string, next http.HandlerFunc) http.HandlerFunc {
	disabled := expected == ""
	devAllow := os.Getenv("PASTAAY_DEV_ALLOW_NO_TOKEN") == "1"
	return func(w http.ResponseWriter, r *http.Request) {
		// Reject browser-driven CSRF
		if r.Method == http.MethodPost && !isSameOriginOrJSON(r) {
			http.Error(w, "forbidden (CSRF check failed)", http.StatusForbidden)
			return
		}

		if disabled {
			if !devAllow {
				http.Error(w, "engine token not configured: refusing to serve protected route", http.StatusServiceUnavailable)
				return
			}
			log.Printf("[Pastaay-Console] DEV: unauthenticated %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
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

// isSameOriginOrJSON validates Origin/Referer/Content-Type for form requests.
func isSameOriginOrJSON(r *http.Request) bool {
	ct := strings.ToLower(strings.TrimSpace(strings.SplitN(r.Header.Get("Content-Type"), ";", 2)[0]))
	if ct == "application/json" || ct == "application/yaml" || ct == "application/x-yaml" {
		return true
	}
	host := r.Host
	if o := r.Header.Get("Origin"); o != "" {
		if u, err := url.Parse(o); err == nil && strings.EqualFold(u.Host, host) {
			return true
		}
		return false
	}
	if rfr := r.Header.Get("Referer"); rfr != "" {
		if u, err := url.Parse(rfr); err == nil && strings.EqualFold(u.Host, host) {
			return true
		}
		return false
	}
	// No Origin/Referer
	return false
}

func renderHTML(tmpl *template.Template, w http.ResponseWriter, page string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout.html", map[string]string{"Page": page}); err != nil {
		log.Printf("[Pastaay-Web] template render error (%s): %v", page, err)
	}
}

// handleOracle proxies AI requests using server-side keys.
func handleOracle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Provider, Model, Intensity, Prompt, Context string
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxAPIRequestBytes)).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if !allowedProvs[strings.ToLower(req.Provider)] {
		http.Error(w, "unsupported provider", http.StatusBadRequest)
		return
	}
	if req.Model != "" && !modelNameRe.MatchString(req.Model) {
		http.Error(w, "invalid model name", http.StatusBadRequest)
		return
	}

	apiKey := providerKey(req.Provider)
	if apiKey == "" {
		http.Error(w, "provider not configured on engine", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	resp, err := oracle.AskOracleCtx(r.Context(), req.Provider, apiKey, req.Model, req.Intensity, req.Prompt, req.Context)
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
		var policies = []config.Policy{}

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

func isInternalIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		if v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
			return true
		}
	}
	return false
}

// safeProbeClient is a per-request constructor
func newSafeProbeClient(ip net.IP, port string) *http.Client {
	pinned := net.JoinHostPort(ip.String(), port)
	dialer := &net.Dialer{Timeout: 3 * time.Second}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			// Ignore the caller-supplied addr and dial the pinned IP instead.
			return dialer.DialContext(ctx, network, pinned)
		},
		ResponseHeaderTimeout: probeTimeout,
		DisableKeepAlives:     true,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   probeTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return errors.New("pastaay: redirects are disabled for probe targets")
		},
	}
}

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

	host := u.Hostname()
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}

	result := map[string]interface{}{}
	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), probeTimeout)
	defer cancel()

	// Single resolution pass — pick the first public IP.
	ips, lerr := (&net.Resolver{}).LookupIPAddr(ctx, host)
	if lerr != nil || len(ips) == 0 {
		result["status"] = 0
		result["error"] = fmt.Sprintf("dns resolve: %v", lerr)
		_ = json.NewEncoder(w).Encode(result)
		return
	}
	var pinned net.IP
	for _, ip := range ips {
		if !isInternalIP(ip.IP) {
			pinned = ip.IP
			break
		}
	}
	if pinned == nil {
		result["status"] = 0
		result["error"] = "pastaay: refused to dial only-internal hostname"
		_ = json.NewEncoder(w).Encode(result)
		return
	}

	probeReq, err := http.NewRequestWithContext(ctx, http.MethodGet, req.URL, nil)
	if err != nil {
		result["status"] = 0
		result["error"] = err.Error()
		_ = json.NewEncoder(w).Encode(result)
		return
	}

	client := newSafeProbeClient(pinned, port)
	defer client.CloseIdleConnections()

	start := time.Now()
	resp, err := client.Do(probeReq)
	elapsed := time.Since(start).Milliseconds()
	result["elapsed_ms"] = elapsed

	if err != nil {
		result["status"] = 0
		result["error"] = err.Error()
		_ = json.NewEncoder(w).Encode(result)
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, probeMaxBodyBytes))

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

// RegisterHandlers wires console + admin API routes onto mux.
func RegisterHandlers(ctx context.Context, mux *http.ServeMux, mgr *config.Manager) {
	adminToken := os.Getenv("PASTAAY_WEBHOOK_TOKEN")

	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		if restConfig, err := rest.InClusterConfig(); err == nil {
			if clientset, csErr := kubernetes.NewForConfig(restConfig); csErr == nil {
				go watchAndStreamPods(ctx, clientset, getEnv("PASTAAY_K8S_NAMESPACE", "default"))
			}
		}
	}

	tmpl := template.Must(template.ParseFS(TemplatesFS, "templates/*.html"))
	mux.Handle("/static/", http.FileServer(http.FS(StaticFS)))
	mux.Handle("/console/docs/raw/", http.StripPrefix("/console/docs/raw/", http.FileServer(http.FS(docs.FS))))

	mux.HandleFunc("/console", func(w http.ResponseWriter, r *http.Request) { renderHTML(tmpl, w, "dashboard") })
	mux.HandleFunc("/console/docs", func(w http.ResponseWriter, r *http.Request) { renderHTML(tmpl, w, "docs") })
	mux.HandleFunc("/console/builder", func(w http.ResponseWriter, r *http.Request) { renderHTML(tmpl, w, "builder") })

	mux.HandleFunc("/console/api/metrics", requireConsoleToken(adminToken, handleMetrics))
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
