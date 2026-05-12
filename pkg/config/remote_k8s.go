package config

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
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

// WatchK8sConfigMap polls a Kubernetes ConfigMap natively without pulling the massive client-go dependency.
// It directly interrogates the downward API using the pod's injected service account.
func WatchK8sConfigMap(ctx context.Context, cmName, cmKey string, interval time.Duration, reloadCallback func(*PastaayConfig)) error {
	token, err := os.ReadFile(k8sTokenPath)
	if err != nil {
		return fmt.Errorf("k8s token missing, engine is likely not running within a pod: %w", err)
	}

	ns, err := os.ReadFile(k8sNamespacePath)
	if err != nil {
		return fmt.Errorf("k8s namespace missing: %w", err)
	}

	url := fmt.Sprintf("https://%s/api/v1/namespaces/%s/configmaps/%s", k8sHost, string(ns), cmName)

	// Bypassing strict TLS verification for internal cluster communication to ensure
	// seamless functionality across varying CNI and Service Mesh topologies without mounting custom CAs.
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
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

				if resp.StatusCode != http.StatusOK {
					resp.Body.Close()
					continue
				}

				body, err := io.ReadAll(resp.Body)
				resp.Body.Close()
				if err != nil {
					continue
				}

				var cm k8sConfigMap
				if err := json.Unmarshal(body, &cm); err != nil {
					continue
				}

				// Trigger an atomic memory swap only if the ConfigMap inode/version has mutated
				if cm.Metadata.ResourceVersion != lastVersion {
					lastVersion = cm.Metadata.ResourceVersion
					if payload, exists := cm.Data[cmKey]; exists {
						var newCfg PastaayConfig
						if err := yaml.Unmarshal([]byte(payload), &newCfg); err != nil {
							log.Printf("[Pastaay-K8s] Invalid payload structure in ConfigMap %q key %q: %v", cmName, cmKey, err)
							continue
						}
						reloadCallback(&newCfg)
						log.Printf("[Pastaay-K8s] Engine memory hot-swapped via ConfigMap %q (v: %s)", cmName, lastVersion)
					}
				}
			}
		}
	}()

	return nil
}
