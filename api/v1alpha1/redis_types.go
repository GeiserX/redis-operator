/*
Copyright 2025.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RedisSpec defines the desired state of Redis
type RedisSpec struct {
	// Number of Redis instances to deploy
	// +kubebuilder:validation:Minimum=1
	Replicas int32 `json:"replicas"`
	// Docker image to deploy, defaults to bitnami/redis:8.0
	// +optional
	Image string `json:"image,omitempty"`
	// Resources defines resource requests and limits
	// +optional
	Resources ResourceSpec `json:"resources,omitempty"`
	// Add Probe Config
	// +optional
	Probes ProbeSpec `json:"probes,omitempty"`
}

// ResourceSpec defines CPU and memory requests/limits
type ResourceSpec struct {
	Requests ResourceList `json:"requests,omitempty"`
	Limits   ResourceList `json:"limits,omitempty"`
}

// ResourceList details resource units
type ResourceList struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

// ProbeSpec defines settings for readiness/liveness probes
type ProbeSpec struct {
	// +optional
	Readiness ProbeConfig `json:"readiness,omitempty"`
	// +optional
	Liveness ProbeConfig `json:"liveness,omitempty"`
}

// ProbeConfig defines individual probe settings
type ProbeConfig struct {
	// Custom command to execute for the probe
	// +optional
	Command []string `json:"command,omitempty"`
	// +optional
	InitialDelaySeconds int32 `json:"initialDelaySeconds,omitempty"`
	// +optional
	PeriodSeconds int32 `json:"periodSeconds,omitempty"`
	// +optional
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`
	// +optional
	FailureThreshold int32 `json:"failureThreshold,omitempty"`
}

// RedisStatus defines the observed state of Redis
type RedisStatus struct {
	Nodes      []string           `json:"nodes,omitempty"`
	Status     string             `json:"status,omitempty"`
	Message    string             `json:"message,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=red
// +kubebuilder:subresource:status
// Redis is the Schema for the redis API
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Pass",type=string,JSONPath=".status.conditions[?(@.type=='PasswordGenerated')].status"
// +kubebuilder:printcolumn:name="Deploy",type=string,JSONPath=".status.conditions[?(@.type=='DeploymentReady')].status"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"
type Redis struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedisSpec   `json:"spec,omitempty"`
	Status RedisStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// RedisList contains a list of Redis
type RedisList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Redis `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Redis{}, &RedisList{})
}

// Setting defaults. Prefer it here over the controller or webhook for simplicity.
func (r *Redis) SetDefaults() {
	if r.Spec.Image == "" {
		r.Spec.Image = "bitnami/redis:8.0"
	}

	if r.Spec.Replicas == 0 {
		r.Spec.Replicas = 1
	}

	if r.Spec.Resources.Requests.CPU == "" {
		r.Spec.Resources.Requests.CPU = "100m"
	}
	if r.Spec.Resources.Requests.Memory == "" {
		r.Spec.Resources.Requests.Memory = "128Mi"
	}
	if r.Spec.Resources.Limits.CPU == "" {
		r.Spec.Resources.Limits.CPU = "250m"
	}
	if r.Spec.Resources.Limits.Memory == "" {
		r.Spec.Resources.Limits.Memory = "256Mi"
	}
	// Liveness default command
	if len(r.Spec.Probes.Liveness.Command) == 0 {
		r.Spec.Probes.Liveness.Command = []string{"sh", "-c", `redis-cli -a "$REDIS_PASSWORD" PING`}
	}
	// Liveness timing defaults
	if r.Spec.Probes.Liveness.InitialDelaySeconds == 0 {
		r.Spec.Probes.Liveness.InitialDelaySeconds = 20
	}
	if r.Spec.Probes.Liveness.PeriodSeconds == 0 {
		r.Spec.Probes.Liveness.PeriodSeconds = 10
	}
	if r.Spec.Probes.Liveness.TimeoutSeconds == 0 {
		r.Spec.Probes.Liveness.TimeoutSeconds = 3
	}
	if r.Spec.Probes.Liveness.FailureThreshold == 0 {
		r.Spec.Probes.Liveness.FailureThreshold = 3
	}

	// Readiness default command
	if len(r.Spec.Probes.Readiness.Command) == 0 {
		r.Spec.Probes.Readiness.Command = []string{"sh", "-c", `redis-cli -a "$REDIS_PASSWORD" SET readiness_probe OK`}
	}
	// Readiness timing defaults
	if r.Spec.Probes.Readiness.InitialDelaySeconds == 0 {
		r.Spec.Probes.Readiness.InitialDelaySeconds = 5
	}
	if r.Spec.Probes.Readiness.PeriodSeconds == 0 {
		r.Spec.Probes.Readiness.PeriodSeconds = 15
	}
	if r.Spec.Probes.Readiness.TimeoutSeconds == 0 {
		r.Spec.Probes.Readiness.TimeoutSeconds = 4
	}
	if r.Spec.Probes.Readiness.FailureThreshold == 0 {
		r.Spec.Probes.Readiness.FailureThreshold = 3
	}
}
