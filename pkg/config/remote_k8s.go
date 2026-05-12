package config

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	k8sTokenPath     = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	k8sNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	k8sHost          = "kubernetes.default.svc"
)

type k8sConfigMap struct {
	Metadata struct {
		ResourceVersion string `json:"resourceVersion"`
	} `json:"metadata"`
	Data map[string]string `json:"data"`
}

// WatchK8sConfigMap polls a Kubernetes ConfigMap natively with aggressive network timeouts
// to prevent goroutine leaks during context cancellation.
func WatchK8sConfigMap(ctx context.Context, cmName, cmKey string, interval time.Duration, reloadCallback func(*PastaayConfig)) error {
	token, err := os.ReadFile(k8sTokenPath)
	if err != nil {
		return fmt.Errorf("k8s token missing: %w", err)
	}

	ns, err := os.ReadFile(k8sNamespacePath)
	if err != nil {
		return fmt.Errorf("k8s namespace missing: %w", err)
	}

	url := fmt.Sprintf("https://%s/api/v1/namespaces/%s/configmaps/%s", k8sHost, string(ns), cmName)

	// Prevents the watcher from becoming a zombie goroutine
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ResponseHeaderTimeout: 5 * time.Second,
		IdleConnTimeout:       60 * time.Second,
		MaxIdleConns:          10,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
	}

	go func() {
		var lastVersion string
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
				if err != nil {
					continue
				}
				req.Header.Set("Authorization", "Bearer "+string(token))

				resp, err := client.Do(req)
				if err != nil {
					continue
				}

				var cm k8sConfigMap
				// Use LimitReader to prevent massive response payloads
				err = json.NewDecoder(io.LimitReader(resp.Body, 5<<20)).Decode(&cm)
				resp.Body.Close()
				if err != nil {
					continue
				}

				if cm.Metadata.ResourceVersion != lastVersion {
					lastVersion = cm.Metadata.ResourceVersion
					if payload, exists := cm.Data[cmKey]; exists {
						var newCfg PastaayConfig
						if err := yaml.Unmarshal([]byte(payload), &newCfg); err != nil {
							log.Printf("[Pastaay-K8s] Invalid structure in CM %q: %v", cmName, err)
							continue
						}

						if err := newCfg.Validate(); err != nil {
							log.Printf("[Pastaay-K8s] Validation failed for CM %q: %v", cmName, err)
							continue
						}

						reloadCallback(&newCfg)
						log.Printf("[Pastaay-K8s] Engine updated via ConfigMap %q (v:%s)", cmName, lastVersion)
					}
				}
			}
		}
	}()

	return nil
}
