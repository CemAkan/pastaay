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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ChaosPolicySpec defines the desired state of ChaosPolicy
type ChaosPolicySpec struct {
	// Protocol type: http, sql, grpc, redis, mongo, kafka, rabbitmq, resource
	// +kubebuilder:validation:Enum=http;sql;grpc;redis;mongo;kafka;rabbitmq;resource
	Type string `json:"type"`

	// Target endpoint, query, or topic (e.g., "/api/v1/users", "SELECT 1")
	Target string `json:"target"`

	// Probability of injecting latency (0.0 to 1.0)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1
	// +optional
	LatencyChance float64 `json:"latencyChance,omitempty"`

	// Delay duration (e.g., "500ms", "2s")
	// +optional
	LatencyDuration string `json:"latencyDuration,omitempty"`

	// Probability of injecting a synthetic error (0.0 to 1.0)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1
	// +optional
	ErrorChance float64 `json:"errorChance,omitempty"`

	// Custom HTTP/gRPC status code (e.g., 500, 404, 13)
	// +optional
	ErrorCode int `json:"errorCode,omitempty"`

	// Custom error message body to be returned
	// +optional
	ErrorBody string `json:"errorBody,omitempty"`

	// Forcefully drop the connection (requires target: all or database)
	// +optional
	DropConnection bool `json:"dropConnection,omitempty"`
}

// ChaosPolicyStatus defines the observed state of ChaosPolicy
type ChaosPolicyStatus struct {
	// Represents the overall state of the policy (e.g., Applied, Failed)
	// +optional
	Phase string `json:"phase,omitempty"`

	// Timestamp of the last successful synchronization with the engine
	// +optional
	LastAppliedTime *metav1.Time `json:"lastAppliedTime,omitempty"`

	// conditions represent the current state of the ChaosPolicy resource
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ChaosPolicy is the Schema for the chaospolicies API
type ChaosPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ChaosPolicySpec   `json:"spec,omitempty"`
	Status ChaosPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ChaosPolicyList contains a list of ChaosPolicy
type ChaosPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ChaosPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ChaosPolicy{}, &ChaosPolicyList{})
}
