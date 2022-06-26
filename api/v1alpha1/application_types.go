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

// ApplicationRef defines a reference to an application
type ApplicationRef struct {
	// Namespace is the namespace in which the application resides in
	Namespace string `json:"namespace,omitempty"`

	// Name is the name of the application
	Name string `json:"name"`
}

// ApplicationSpec defines the desired state of Application
type ApplicationSpec struct {
	// PodSpec describes the pods that will be created
	PodSpec corev1.PodSpec `json:"podSpec"`

	// Dependencies defines the list of applications this application depends on
	Dependencies []ApplicationRef `json:"dependencies,omitempty"`
}

// ApplicationStatus defines the observed state of Application
type ApplicationStatus struct {
	// MissingDependencies describes the number of dependencies that are not there in the cluster
	MissingDependencies int `json:"missingDependencies"`

	// Dependents defines the list of applications which depends on this
	Dependents []ApplicationRef `json:"dependents,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Application is the Schema for the applications API
type Application struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApplicationSpec   `json:"spec,omitempty"`
	Status ApplicationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ApplicationList contains a list of Application
type ApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Application `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Application{}, &ApplicationList{})
}
