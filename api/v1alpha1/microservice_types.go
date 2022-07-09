/*
 * Copyright (c) 2022, Nadun De Silva. All Rights Reserved.
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *   http://www.apache.org/licenses/LICENSE-2.0
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MicroserviceRef defines a reference to an microservice
type MicroserviceRef struct {
	// Namespace is the namespace in which the microservice resides in
	Namespace string `json:"namespace,omitempty"`

	// Name is the name of the microservice
	Name string `json:"name"`
}

// MicroserviceSpec defines the desired state of Microservice
type MicroserviceSpec struct {
	// Replicas specifies the replicas count of the deployment
	Replicas *int32 `json:"replicas,omitempty"`

	// PodSpec describes the pods that will be created
	PodSpec corev1.PodSpec `json:"podSpec"`

	// Dependencies defines the list of microservices this microservice depends on
	Dependencies []MicroserviceRef `json:"dependencies,omitempty"`
}

// MicroserviceStatus defines the observed state of Microservice
type MicroserviceStatus struct {
	// Replicas describes the current actual replica count
	Replicas int32 `json:"replicas"`

	// Selector describes the string form of the selector
	Selector string `json:"selector"`

	// MissingDependencies describes the number of dependencies that are not there in the cluster
	MissingDependencies []MicroserviceRef `json:"missingDependencies"`

	// Dependents defines the list of microservices which depends on this
	Dependents []MicroserviceRef `json:"dependents,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas,selectorpath=.status.selector

// Microservice is the Schema for the microservices API
type Microservice struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MicroserviceSpec   `json:"spec,omitempty"`
	Status MicroserviceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MicroserviceList contains a list of Microservice
type MicroserviceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Microservice `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Microservice{}, &MicroserviceList{})
}
