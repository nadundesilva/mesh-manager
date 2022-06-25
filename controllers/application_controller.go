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

package controllers

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	meshmanagerv1alpha1 "github.com/nadundesilva/mesh-manager/api/v1alpha1"
)

// ApplicationReconciler reconciles a Application object
type ApplicationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=mesh-manager.nadundesilva.github.io,resources=applications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mesh-manager.nadundesilva.github.io,resources=applications/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mesh-manager.nadundesilva.github.io,resources=applications/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Application object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *ApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("application", req.NamespacedName)
	ctx = log.IntoContext(ctx, logger)
	logger.Info("Reconciling application")

	application := &meshmanagerv1alpha1.Application{}
	if err := r.Client.Get(ctx, req.NamespacedName, application); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		} else {
			return ctrl.Result{}, fmt.Errorf("failed to get existing application: %+w", err)
		}
	}

	labels := map[string]string{
		"application": req.Name,
	}
	for k, v := range application.Labels {
		labels[k] = v
	}

	if err := r.reconcileDependantSlices(ctx, req, application, labels); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile application dependency slices: %+w", err)
	}
	if len(application.Spec.Dependencies) == 0 || application.Status.MissingDependencies == 0 {
		if err := r.reconcileDeployment(ctx, req, application, labels); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to reconcile application deployment: %+w", err)
		}
		if err := r.reconcileService(ctx, req, application, labels); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to reconcile application service: %+w", err)
		}
	} else {
		logger.Info("Application resources not created since it contains unmet dependencies")
	}

	if err := r.Client.Update(ctx, application); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update existing application: %+w", err)
	}
	return ctrl.Result{}, nil
}

func (r *ApplicationReconciler) reconcileDeployment(ctx context.Context, req ctrl.Request,
	application *meshmanagerv1alpha1.Application, labels map[string]string) error {
	deployment := &appsv1.Deployment{}
	err := r.Client.Get(ctx, req.NamespacedName, deployment)
	var isNotFound bool
	if err != nil {
		isNotFound = errors.IsNotFound(err)
		if !isNotFound {
			return fmt.Errorf("failed to get existing application deployment: %+w", err)
		}
	}

	deployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: req.Namespace,
			Name:      req.Name,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: application.Spec.PodSpec,
			},
		},
	}
	if err := ctrl.SetControllerReference(application, deployment, r.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference to application deployment: %+w", err)
	}

	if isNotFound {
		if err := r.Client.Create(ctx, deployment); err != nil {
			return fmt.Errorf("failed to create new application deployment: %+w", err)
		}
	} else {
		if err := r.Client.Update(ctx, deployment); err != nil {
			return fmt.Errorf("failed to update existing application deployment: %+w", err)
		}
	}
	return nil
}

func (r *ApplicationReconciler) reconcileService(ctx context.Context, req ctrl.Request,
	application *meshmanagerv1alpha1.Application, labels map[string]string) error {
	service := &corev1.Service{}
	err := r.Client.Get(ctx, req.NamespacedName, service)
	var isNotFound bool
	if err != nil {
		isNotFound = errors.IsNotFound(err)
		if !isNotFound {
			return fmt.Errorf("failed to get existing application service: %+w", err)
		}
	}

	ports := []corev1.ServicePort{}
	for _, container := range application.Spec.PodSpec.Containers {
		for _, port := range container.Ports {
			ports = append(ports, corev1.ServicePort{
				Name:     port.Name,
				Protocol: port.Protocol,
				Port:     port.ContainerPort,
				TargetPort: (func() intstr.IntOrString {
					if port.Name == "" {
						return intstr.FromInt(int(port.ContainerPort))
					} else {
						return intstr.FromString(port.Name)
					}
				})(),
			})
		}
	}
	service = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: req.Namespace,
			Name:      req.Name,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports:    ports,
		},
	}
	if err := ctrl.SetControllerReference(application, service, r.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference to application service: %+w", err)
	}

	if isNotFound {
		if err := r.Client.Create(ctx, service); err != nil {
			return fmt.Errorf("failed to create new application service: %+w", err)
		}
	} else {
		if err := r.Client.Update(ctx, service); err != nil {
			return fmt.Errorf("failed to update existing application service: %+w", err)
		}
	}
	return nil
}

func (r *ApplicationReconciler) reconcileDependantSlices(ctx context.Context, req ctrl.Request,
	application *meshmanagerv1alpha1.Application, labels map[string]string) error {
	dependantSlice := &meshmanagerv1alpha1.DependantSlice{}
	err := r.Client.Get(ctx, req.NamespacedName, dependantSlice)
	if err != nil {
		if errors.IsNotFound(err) {
			dependantSlice := &meshmanagerv1alpha1.DependantSlice{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: req.Namespace,
					Name:      req.Name,
					Labels:    labels,
				},
				Spec: meshmanagerv1alpha1.DependantSliceSpec{},
			}
			if err := ctrl.SetControllerReference(application, dependantSlice, r.Scheme); err != nil {
				return fmt.Errorf("failed to set controller reference to new dependant slice: %+w", err)
			}
			if err := r.Client.Create(ctx, dependantSlice); err != nil {
				return fmt.Errorf("failed to create new application dependant slice: %+w", err)
			}
		} else {
			return fmt.Errorf("failed to get existing application dependant slice: %+w", err)
		}
	} else {
		if err := ctrl.SetControllerReference(application, dependantSlice, r.Scheme); err != nil {
			return fmt.Errorf("failed to set controller reference to existing dependant slice: %+w", err)
		}
		if err := r.Client.Update(ctx, dependantSlice); err != nil {
			return fmt.Errorf("failed to update existing application dependant slice: %+w", err)
		}
	}

	applicationRef := meshmanagerv1alpha1.ApplicationRef{
		Namespace: application.Namespace,
		Name:      application.Name,
	}
	application.Status.MissingDependencies = 0
	for _, dependency := range application.Spec.Dependencies {
		dependencyDependantSlice := &meshmanagerv1alpha1.DependantSlice{}
		dependencyName := apitypes.NamespacedName{
			Namespace: func() string {
				if dependency.Namespace == "" {
					return application.GetNamespace()
				} else {
					return dependency.Namespace
				}
			}(),
			Name: dependency.Name,
		}

		err := r.Client.Get(ctx, dependencyName, dependencyDependantSlice)
		var isDependantSliceNotFound bool
		if err != nil {
			isDependantSliceNotFound = errors.IsNotFound(err)
			if isDependantSliceNotFound {
				dependencyDependantSlice = &meshmanagerv1alpha1.DependantSlice{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: dependencyName.Namespace,
						Name:      dependencyName.Name,
						Labels:    labels,
					},
					Spec: meshmanagerv1alpha1.DependantSliceSpec{
						Dependants: []meshmanagerv1alpha1.ApplicationRef{},
					},
				}
			} else {
				return fmt.Errorf("failed to get existing dependency dependant slice: %+w", err)
			}
		}

		dependencyApplication := &meshmanagerv1alpha1.Application{}
		err = r.Client.Get(ctx, dependencyName, dependencyApplication)
		if err != nil {
			if errors.IsNotFound(err) {
				application.Status.MissingDependencies += 1
			} else {
				return fmt.Errorf("failed to get existing dependency: %+w", err)
			}
		}

		isDependencyLinked := false
		for _, dependant := range dependencyDependantSlice.Spec.Dependants {
			if dependant.Namespace == applicationRef.Namespace && dependant.Name == applicationRef.Name {
				isDependencyLinked = true
			}
		}
		if !isDependencyLinked {
			dependencyDependantSlice.Spec.Dependants = append(dependencyDependantSlice.Spec.Dependants, applicationRef)
		}

		if isDependantSliceNotFound {
			if err := r.Client.Create(ctx, dependencyDependantSlice); err != nil {
				return fmt.Errorf("failed to create new dependency dependant slice: %+w", err)
			}
		} else {
			if len(dependencyDependantSlice.Spec.Dependants) == 0 {
				if err := r.Client.Delete(ctx, dependencyDependantSlice,
					client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil {
					return fmt.Errorf("failed to delete existing dependency dependant slice without dependants: %+w", err)
				}
			} else {
				if err := r.Client.Update(ctx, dependencyDependantSlice); err != nil {
					return fmt.Errorf("failed to update existing dependency dependant slice: %+w", err)
				}
			}
		}
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&meshmanagerv1alpha1.Application{}).
		Complete(r)
}
