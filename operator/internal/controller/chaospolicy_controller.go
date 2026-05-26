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

const chaosPolicyFinalizer = "chaos.pastaay.io/finalizer"

// Shared HTTP client with bounded timeout to prevent reconcile-queue starvation.
var sharedEngineClient = &http.Client{Timeout: 10 * time.Second}

// postToEngine sends a JSON payload to the engine webhook with proper context cancellation and optional bearer-token authentication.
func postToEngine(ctx context.Context, url string, payload []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if tok := os.Getenv("PASTAAY_WEBHOOK_TOKEN"); tok != "" {
		req.Header.Set("X-Pastaay-Token", tok)
	}
	return sharedEngineClient.Do(req)
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

	if policy.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(&policy, chaosPolicyFinalizer) {
			l.Info("ChaosPolicy deletion detected. Cleaning Engine memory...", "policy", policy.Name)

			rollbackPayload := map[string]interface{}{
				"version":  1,
				"policies": []interface{}{},
			}
			payloadBytes, err := json.Marshal(rollbackPayload)
			if err != nil {
				return ctrl.Result{}, err
			}
			resp, err := postToEngine(ctx, r.EngineURL, payloadBytes)
			if err != nil {
				l.Error(err, "Failed to send deletion cleanup to Engine, retrying...", "url", r.EngineURL)
				return ctrl.Result{}, err
			}
			defer resp.Body.Close()

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

	// Autonomous Rollback Logic (If time has passed naturally)
	if experimentDuration > 0 && policy.Status.LastAppliedTime != nil {
		expirationTime := policy.Status.LastAppliedTime.Time.Add(experimentDuration)

		if time.Now().After(expirationTime) {
			l.Info("Chaos duration expired. Initiating autonomous rollback.", "policy", policy.Name)

			rollbackPayload := map[string]interface{}{
				"version":  1,
				"policies": []interface{}{},
			}

			payloadBytes, err := json.Marshal(rollbackPayload)
			if err != nil {
				return ctrl.Result{}, err
			}
			resp, err := postToEngine(ctx, r.EngineURL, payloadBytes)
			if err != nil {
				return ctrl.Result{}, err
			}
			defer resp.Body.Close()

			// Update Status to Expired
			policy.Status.Phase = "Expired"
			if err := r.Status().Update(ctx, &policy); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
	}

	// Standard Chaos Injection Pipeline
	l.Info("Reconciling ChaosPolicy", "name", policy.Name, "type", policy.Spec.Type)
	wrappedPayload := map[string]interface{}{
		"version":  1,
		"policies": []interface{}{policy.Spec},
	}

	payload, err := json.Marshal(wrappedPayload)
	if err != nil {
		return ctrl.Result{}, err
	}

	resp, err := postToEngine(ctx, r.EngineURL, payload)
	if err != nil {
		l.Error(err, "Failed to connect to Pastaay Engine", "url", r.EngineURL)
		return ctrl.Result{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		l.Info("Engine rejected the policy", "code", resp.StatusCode)
		return ctrl.Result{RequeueAfter: time.Second * 30}, nil
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

func (r *ChaosPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&chaosv1.ChaosPolicy{}).
		Complete(r)
}
