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

// RedisStatus defines the observed state of Redis
type RedisStatus struct {
	Nodes   []string `json:"nodes,omitempty"`
	Status  string   `json:"status,omitempty"`
	Message string   `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=red
// +kubebuilder:subresource:status
// Redis is the Schema for the redis API
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
}
