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
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var microservicelog = logf.Log.WithName("microservice-resource")

func (r *Microservice) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-mesh-manager-nadundesilva-github-io-v1alpha1-microservice,mutating=true,failurePolicy=fail,sideEffects=None,groups=mesh-manager.nadundesilva.github.io,resources=microservices,verbs=create;update,versions=v1alpha1,name=mmicroservice.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &Microservice{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Microservice) Default() {
	microservicelog.Info("default", "name", r.Name)

	if r.Spec.Replicas == nil {
		r.Spec.Replicas = pointer.Int32(1)
	}
	for _, dependency := range r.Spec.Dependencies {
		if dependency.Namespace == "" {
			dependency.Namespace = r.GetNamespace()
		}
	}
}

//+kubebuilder:webhook:path=/validate-mesh-manager-nadundesilva-github-io-v1alpha1-microservice,mutating=false,failurePolicy=fail,sideEffects=None,groups=mesh-manager.nadundesilva.github.io,resources=microservices,verbs=create;update;delete,versions=v1alpha1,name=vmicroservice.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Microservice{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Microservice) ValidateCreate() error {
	microservicelog.Info("validate create", "name", r.Name)
	return r.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Microservice) ValidateUpdate(old runtime.Object) error {
	microservicelog.Info("validate update", "name", r.Name)
	return r.validate()
}

func (r *Microservice) validate() error {
	if *r.Spec.Replicas < 0 {
		return fmt.Errorf("microservice replica count cannot be below zero")
	}

	existingPorts := []int32{}
	for _, container := range r.Spec.PodSpec.Containers {
		for _, port := range container.Ports {
			for _, existingPort := range existingPorts {
				if existingPort == port.ContainerPort {
					return fmt.Errorf("microservice cannot contain duplicated port %d", port.ContainerPort)
				}
			}
			existingPorts = append(existingPorts, port.ContainerPort)
		}
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Microservice) ValidateDelete() error {
	microservicelog.Info("validate delete", "name", r.Name)
	if len(r.Status.Dependents) > 0 {
		return fmt.Errorf("unable to delete while dependants %+v exists", r.Status.Dependents)
	}
	return nil
}
