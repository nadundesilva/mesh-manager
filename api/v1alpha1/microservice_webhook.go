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
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var microservicelog = logf.Log.WithName("microservice-resource")

func (r *Microservice) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-mesh-manager-nadundesilva-github-io-v1alpha1-microservice,mutating=true,failurePolicy=fail,sideEffects=None,groups=mesh-manager.nadundesilva.github.io,resources=microservices,verbs=create;update,versions=v1alpha1,name=mesh-manager.nadundesilva.github.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &Microservice{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Microservice) Default() {
	microservicelog.Info("default", "name", r.GetName(), "namespace", r.GetNamespace())

	if r.Spec.Replicas == nil {
		r.Spec.Replicas = ptr.To[int32](1)
	}
	for i, dependency := range r.Spec.Dependencies {
		if dependency.Namespace == "" {
			dependency.Namespace = r.GetNamespace()
		}
		r.Spec.Dependencies[i] = dependency
	}
}

//+kubebuilder:webhook:path=/validate-mesh-manager-nadundesilva-github-io-v1alpha1-microservice,mutating=false,failurePolicy=fail,sideEffects=None,groups=mesh-manager.nadundesilva.github.io,resources=microservices,verbs=create;update;delete,versions=v1alpha1,name=vmicroservice.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Microservice{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Microservice) ValidateCreate() (warnings admission.Warnings, err error) {
	microservicelog.Info("validate create", "name", r.GetName(), "namespace", r.GetNamespace())
	return r.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Microservice) ValidateUpdate(old runtime.Object) (warnings admission.Warnings, err error) {
	microservicelog.Info("validate update", "name", r.GetName(), "namespace", r.GetNamespace())
	return r.validate()
}

func (r *Microservice) validate() (warnings admission.Warnings, err error) {
	if *r.Spec.Replicas < 0 {
		return admission.Warnings{}, fmt.Errorf("microservice replica count cannot be below zero")
	}

	duplicatedPorts := []int32{}
	existingPorts := []int32{}
	for _, container := range r.Spec.PodSpec.Containers {
		for _, port := range container.Ports {
			for _, existingPort := range existingPorts {
				if existingPort == port.ContainerPort {
					duplicatedPorts = append(duplicatedPorts, port.ContainerPort)
				}
			}
			existingPorts = append(existingPorts, port.ContainerPort)
		}
	}
	if len(duplicatedPorts) > 0 {
		return admission.Warnings{}, fmt.Errorf("microservice cannot contain duplicated port(s) %v", duplicatedPorts)
	}
	return admission.Warnings{}, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Microservice) ValidateDelete() (warnings admission.Warnings, err error) {
	microservicelog.Info("validate delete", "name", r.GetName(), "namespace", r.GetNamespace())
	if len(r.Status.Dependents) > 0 {
		return admission.Warnings{}, fmt.Errorf("unable to delete while dependants %+v exist", r.Status.Dependents)
	}
	return admission.Warnings{}, nil
}
