/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	chaosv1 "github.com/CemAkan/pastaay/operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	chaosPolicyFinalizer = "chaos.pastaay.io/finalizer"
	// maxBodyBytes bounds the engine response we read on POST so a
	// hostile/buggy engine cannot drive operator OOM.
	maxBodyBytes = 64 << 10
)

// Shared HTTP client with bounded timeout to prevent reconcile-queue starvation.
var sharedEngineClient = &http.Client{Timeout: 10 * time.Second}

// postToEngine sends a JSON payload to the engine webhook with proper context
// cancellation and optional bearer-token authentication. Returns status, body
// snippet, and error.
func postToEngine(ctx context.Context, url string, payload []byte) (int, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if tok := os.Getenv("PASTAAY_WEBHOOK_TOKEN"); tok != "" {
		req.Header.Set("X-Pastaay-Token", tok)
	}
	resp, err := sharedEngineClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	return resp.StatusCode, string(body), nil
}

type ChaosPolicyReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	EngineURL string
}

// +kubebuilder:rbac:groups=chaos.pastaay.io,resources=chaospolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=chaos.pastaay.io,resources=chaospolicies/status,verbs=get;update;patch

func (r *ChaosPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	// Fetch active policy from Cluster
	var policy chaosv1.ChaosPolicy
	if err := r.Get(ctx, req.NamespacedName, &policy); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// ── Deletion path ────────────────────────────────────────────────
	if policy.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(&policy, chaosPolicyFinalizer) {
			l.Info("ChaosPolicy deletion detected. Re‑snapshotting remaining policies.", "policy", policy.Name)

			if err := r.resyncEngineWithRemaining(ctx, policy.Name); err != nil {
				l.Error(err, "Failed to resync Engine on deletion, retrying...", "url", r.EngineURL)
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			}

			controllerutil.RemoveFinalizer(&policy, chaosPolicyFinalizer)
			if err := r.Update(ctx, &policy); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(&policy, chaosPolicyFinalizer) {
		controllerutil.AddFinalizer(&policy, chaosPolicyFinalizer)
		if err := r.Update(ctx, &policy); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Check if the policy has already finished its lifecycle
	if policy.Status.Phase == "Expired" {
		return ctrl.Result{}, nil
	}

	// Parse the duration if provided
	var experimentDuration time.Duration
	if policy.Spec.Duration != "" {
		d, err := time.ParseDuration(policy.Spec.Duration)
		if err == nil {
			experimentDuration = d
		} else {
			l.Error(err, "Invalid duration format. Running indefinitely.")
		}
	}

	// ── Autonomous expiry ────────────────────────────────────────────
	// Re‑snapshot the engine from remaining CRs instead of a blanket
	// wipe so that other active policies survive.
	if experimentDuration > 0 && policy.Status.LastAppliedTime != nil {
		expirationTime := policy.Status.LastAppliedTime.Time.Add(experimentDuration)

		if time.Now().After(expirationTime) {
			l.Info("Chaos duration expired. Re‑snapshotting engine state.", "policy", policy.Name)
			if err := r.resyncEngineWithRemaining(ctx, policy.Name); err != nil {
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			}

			policy.Status.Phase = "Expired"
			if err := r.Status().Update(ctx, &policy); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
	}

	// ── Standard Chaos Injection Pipeline ────────────────────────────
	// Re‑snapshot the whole set so we never accidentally replace the
	// engine's state with a single‑policy view.
	l.Info("Reconciling ChaosPolicy", "name", policy.Name, "type", policy.Spec.Type)
	desired, err := r.collectActivePolicies(ctx)
	if err != nil {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	payload, err := json.Marshal(map[string]interface{}{
		"version":  1,
		"policies": desired,
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	status, body, err := postToEngine(ctx, r.EngineURL, payload)
	if err != nil {
		l.Error(err, "Failed to connect to Pastaay Engine", "url", r.EngineURL)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	if status != http.StatusOK {
		l.Info("Engine rejected the policy", "code", status, "body", truncate(body, 256))
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Update Status & Schedule Rollback Requeue
	if policy.Status.Phase != "Applied" {
		policy.Status.Phase = "Applied"
		now := metav1.Now()
		policy.Status.LastAppliedTime = &now
		if err := r.Status().Update(ctx, &policy); err != nil {
			return ctrl.Result{}, err
		}
	}

	// If there is a duration, wake the controller exactly when it expires
	if experimentDuration > 0 && policy.Status.LastAppliedTime != nil {
		timeLeft := policy.Status.LastAppliedTime.Time.Add(experimentDuration).Sub(time.Now())
		if timeLeft > 0 {
			l.Info("Scheduling autonomous rollback", "requeue_after", timeLeft)
			return ctrl.Result{RequeueAfter: timeLeft}, nil
		}
	}

	return ctrl.Result{}, nil
}

// ── helpers ────────────────────────────────────────────────────────────

// resyncEngineWithRemaining lists all live ChaosPolicy CRs, excludes the
// named policy (the one being deleted/expired), and pushes the result as the
// authoritative engine state.
func (r *ChaosPolicyReconciler) resyncEngineWithRemaining(ctx context.Context, excludeName string) error {
	remaining, err := r.collectActivePoliciesExcluding(ctx, excludeName)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]interface{}{
		"version":  1,
		"policies": remaining,
	})
	if err != nil {
		return err
	}
	status, body, err := postToEngine(ctx, r.EngineURL, payload)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("engine rejected resync: %d %s", status, truncate(body, 256))
	}
	return nil
}

func (r *ChaosPolicyReconciler) collectActivePolicies(ctx context.Context) ([]chaosv1.ChaosPolicySpec, error) {
	return r.collectActivePoliciesExcluding(ctx, "")
}

func (r *ChaosPolicyReconciler) collectActivePoliciesExcluding(ctx context.Context, excludeName string) ([]chaosv1.ChaosPolicySpec, error) {
	var list chaosv1.ChaosPolicyList
	if err := r.List(ctx, &list); err != nil {
		return nil, err
	}
	out := make([]chaosv1.ChaosPolicySpec, 0, len(list.Items))
	for _, p := range list.Items {
		if p.Name == excludeName {
			continue
		}
		if p.GetDeletionTimestamp() != nil {
			continue
		}
		if p.Status.Phase == "Expired" {
			continue
		}
		out = append(out, p.Spec)
	}
	return out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func (r *ChaosPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&chaosv1.ChaosPolicy{}).
		Complete(r)
}
