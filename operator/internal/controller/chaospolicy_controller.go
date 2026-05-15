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
	"time"

	chaosv1 "github.com/CemAkan/pastaay/operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ChaosPolicyReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	EngineURL string
}

// +kubebuilder:rbac:groups=chaos.pastaay.io,resources=chaospolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=chaos.pastaay.io,resources=chaospolicies/status,verbs=get;update;patch

func (r *ChaosPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	var policy chaosv1.ChaosPolicy
	if err := r.Get(ctx, req.NamespacedName, &policy); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	l.Info("Reconciling ChaosPolicy", "name", policy.Name, "type", policy.Spec.Type)

	wrappedPayload := map[string]interface{}{
		"version":  1,
		"policies": []interface{}{policy.Spec},
	}

	payload, err := json.Marshal(wrappedPayload)
	if err != nil {
		return ctrl.Result{}, err
	}

	resp, err := http.Post(r.EngineURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		l.Error(err, "Failed to connect to Pastaay Engine", "url", r.EngineURL)
		return ctrl.Result{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		l.Info("Engine rejected the policy", "code", resp.StatusCode)
		return ctrl.Result{RequeueAfter: time.Second * 30}, nil
	}

	policy.Status.Phase = "Applied"
	now := metav1.Now()
	policy.Status.LastAppliedTime = &now
	if err := r.Status().Update(ctx, &policy); err != nil {
		return ctrl.Result{}, err
	}

	l.Info("Successfully pushed to engine", "policy", policy.Name)
	return ctrl.Result{}, nil
}

func (r *ChaosPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&chaosv1.ChaosPolicy{}).
		Complete(r)
}
