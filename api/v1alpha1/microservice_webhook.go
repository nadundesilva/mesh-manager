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
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (r *Microservice) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		WithDefaulter(&MicroserviceDefaulter{}).
		WithValidator(&MicroserviceValidator{}).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-mesh-manager-nadundesilva-github-io-v1alpha1-microservice,mutating=true,failurePolicy=fail,sideEffects=None,groups=mesh-manager.nadundesilva.github.io,resources=microservices,verbs=create;update,versions=v1alpha1,name=mesh-manager.nadundesilva.github.io,admissionReviewVersions=v1

type MicroserviceDefaulter struct{}

var _ webhook.CustomDefaulter = &MicroserviceDefaulter{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *MicroserviceDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	microservice, ok := obj.(*Microservice)
	if !ok {
		return fmt.Errorf("expected an Microservice object but got %T", obj)
	}
	logger := log.FromContext(ctx).WithValues("name", microservice.GetName(), "namespace", microservice.GetNamespace())

	logger.Info("Defaulting for microservice")

	if microservice.Spec.Replicas == nil {
		microservice.Spec.Replicas = ptr.To[int32](1)
	}
	for i, dependency := range microservice.Spec.Dependencies {
		if dependency.Namespace == "" {
			dependency.Namespace = microservice.GetNamespace()
		}
		microservice.Spec.Dependencies[i] = dependency
	}
	return nil
}

//+kubebuilder:webhook:path=/validate-mesh-manager-nadundesilva-github-io-v1alpha1-microservice,mutating=false,failurePolicy=fail,sideEffects=None,groups=mesh-manager.nadundesilva.github.io,resources=microservices,verbs=create;update;delete,versions=v1alpha1,name=vmicroservice.kb.io,admissionReviewVersions=v1

type MicroserviceValidator struct{}

var _ webhook.CustomValidator = &MicroserviceValidator{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *MicroserviceValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	microservice, ok := obj.(*Microservice)
	if !ok {
		return admission.Warnings{}, fmt.Errorf("expected an Microservice object but got %T", obj)
	}
	logger := log.FromContext(ctx).WithValues("name", microservice.GetName(), "namespace", microservice.GetNamespace())
	ctx = log.IntoContext(ctx, logger)

	logger.Info("Validating microservice create")
	return r.validate(ctx, microservice)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *MicroserviceValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	microservice, ok := newObj.(*Microservice)
	if !ok {
		return admission.Warnings{}, fmt.Errorf("expected an Microservice object but got %T", newObj)
	}
	logger := log.FromContext(ctx).WithValues("name", microservice.GetName(), "namespace", microservice.GetNamespace())
	ctx = log.IntoContext(ctx, logger)

	logger.Info("Validating microservice update")
	return r.validate(ctx, microservice)
}

func (r *MicroserviceValidator) validate(ctx context.Context, microservice *Microservice) (warnings admission.Warnings, err error) {
	if *microservice.Spec.Replicas < 0 {
		return admission.Warnings{}, fmt.Errorf("microservice replica count cannot be below zero")
	}

	duplicatedPorts := []int32{}
	existingPorts := []int32{}
	for _, container := range microservice.Spec.PodSpec.Containers {
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
func (r *MicroserviceValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	microservice, ok := obj.(*Microservice)
	if !ok {
		return admission.Warnings{}, fmt.Errorf("expected an Microservice object but got %T", obj)
	}
	logger := log.FromContext(ctx).WithValues("name", microservice.GetName(), "namespace", microservice.GetNamespace())

	logger.Info("Validating microservice delete")
	if len(microservice.Status.Dependents) > 0 {
		return admission.Warnings{}, fmt.Errorf("unable to delete while dependants %+v exist", microservice.Status.Dependents)
	}
	return admission.Warnings{}, nil
}
