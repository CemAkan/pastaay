package config

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	k8sTokenPath     = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	k8sNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	k8sCAPath        = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	k8sHost          = "kubernetes.default.svc"
)

type k8sConfigMap struct {
	Metadata struct {
		ResourceVersion string `json:"resourceVersion"`
	} `json:"metadata"`
	Data map[string]string `json:"data"`
}

// WatchK8sConfigMap polls K8s API and reports health status to the Manager.
func WatchK8sConfigMap(ctx context.Context, cmName, cmKey string, interval time.Duration, mgr *Manager) error {
	if interval < time.Second {
		interval = time.Second
	}

	_, err := os.ReadFile(k8sTokenPath)
	if err != nil {
		mgr.SetSensorStatus("k8s", "token_missing")
		return fmt.Errorf("k8s token missing: %w", err)
	}

	ns, err := os.ReadFile(k8sNamespacePath)
	if err != nil {
		mgr.SetSensorStatus("k8s", "ns_missing")
		return fmt.Errorf("k8s namespace missing: %w", err)
	}

	caCert, err := os.ReadFile(k8sCAPath)
	if err != nil {
		mgr.SetSensorStatus("k8s", "ca_missing")
		log.Printf("[Pastaay-K8s] FATAL: cannot read service account CA: %v", err)
		return fmt.Errorf("k8s CA missing: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caCert) {
		mgr.SetSensorStatus("k8s", "ca_invalid")
		return fmt.Errorf("pastaay: invalid in-cluster CA bundle")
	}

	url := fmt.Sprintf("https://%s/api/v1/namespaces/%s/configmaps/%s", k8sHost, strings.TrimSpace(string(ns)), cmName)
	mgr.SetSensorStatus("k8s", "initializing")

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs:    pool,
			MinVersion: tls.VersionTLS12,
		},
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ResponseHeaderTimeout: 5 * time.Second,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		MaxIdleConns:          10,
	}

	client := &http.Client{Transport: transport, Timeout: 15 * time.Second}

	go func() {
		var lastVersion string
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Read token freshly
				freshToken, err := os.ReadFile(k8sTokenPath)
				if err != nil {
					mgr.SetSensorStatus("k8s", "token_missing")
					continue
				}

				req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
				if err != nil {
					continue
				}

				req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(freshToken)))

				resp, err := client.Do(req)
				if err != nil {
					mgr.SetSensorStatus("k8s", "api_error")
					continue
				}

				if resp.StatusCode != http.StatusOK {
					_, _ = io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
					mgr.SetSensorStatus("k8s", fmt.Sprintf("http_%d", resp.StatusCode))
					continue
				}

				var cm k8sConfigMap
				err = json.NewDecoder(io.LimitReader(resp.Body, 5<<20)).Decode(&cm)
				resp.Body.Close()

				if err != nil {
					mgr.SetSensorStatus("k8s", "decode_error")
					continue
				}

				if cm.Metadata.ResourceVersion != lastVersion {
					lastVersion = cm.Metadata.ResourceVersion
					if payload, exists := cm.Data[cmKey]; exists {
						var newCfg PastaayConfig
						if err := yaml.Unmarshal([]byte(payload), &newCfg); err != nil {
							mgr.SetSensorStatus("k8s", "yaml_error")
							continue
						}

						if err := newCfg.Validate(); err != nil {
							mgr.SetSensorStatus("k8s", "invalid_config")
							continue
						}

						mgr.Update(&newCfg)
						mgr.SetSensorStatus("k8s", "healthy")
						log.Printf("[Pastaay-K8s] Engine memory hot-swapped (v:%s)", lastVersion)
					}
				}
			}
		}
	}()

	return nil
}
