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
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	meshmanagerv1alpha1 "github.com/nadundesilva/mesh-manager/api/v1alpha1"
)

const applicationFinalizer = "mesh-manager.nadundesilva.github.io/finalizer"

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
	if err := r.Get(ctx, req.NamespacedName, application); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		} else {
			return ctrl.Result{}, fmt.Errorf("failed to get existing application: %+w", err)
		}
	}

	isApplicationMarkedToBeDeleted := application.GetDeletionTimestamp() != nil
	if isApplicationMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(application, applicationFinalizer) {
			isFinalized, err := r.finalize(ctx, req, application)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to finalize application: %+w", err)
			}

			if isFinalized {
				controllerutil.RemoveFinalizer(application, applicationFinalizer)
				err := r.Update(ctx, application)
				if err != nil {
					return ctrl.Result{}, err
				}
			} else {
				return ctrl.Result{RequeueAfter: time.Second * 10}, nil
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(application, applicationFinalizer) {
		controllerutil.AddFinalizer(application, applicationFinalizer)
		err := r.Update(ctx, application)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to add finalizer: %+w", err)
		}
	}

	labels := map[string]string{
		"application": req.Name,
	}
	for k, v := range application.Labels {
		labels[k] = v
	}

	if err := r.reconcileDependencies(ctx, req, application, labels); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile application dependency slices: %+w", err)
	}
	if err := r.Status().Update(ctx, application); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update existing application: %+w", err)
	}

	if len(application.Spec.Dependencies) == 0 || application.Status.MissingDependencies == 0 {
		if err := r.reconcileDeployment(ctx, req, application, labels); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to reconcile application deployment: %+w", err)
		}
		if err := r.reconcileService(ctx, req, application, labels); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to reconcile application service: %+w", err)
		}
		return ctrl.Result{}, nil
	} else {
		logger.Info("Application resources not created since it contains missing dependencies",
			"totalDependenciesCount", len(application.Spec.Dependencies),
			"missingDependenciesCount", application.Status.MissingDependencies)
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}
}

func (r *ApplicationReconciler) reconcileDeployment(ctx context.Context, req ctrl.Request,
	application *meshmanagerv1alpha1.Application, labels map[string]string) error {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: req.Namespace,
			Name:      req.Name,
			Labels:    labels,
		},
	}
	_, err := ctrl.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		deployment.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: labels,
		}
		deployment.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: labels,
			},
			Spec: application.Spec.PodSpec,
		}
		if err := ctrl.SetControllerReference(application, deployment, r.Scheme); err != nil {
			return fmt.Errorf("failed to set controller reference to application deployment: %+w", err)
		}
		return nil
	})
	return err
}

func (r *ApplicationReconciler) reconcileService(ctx context.Context, req ctrl.Request,
	application *meshmanagerv1alpha1.Application, labels map[string]string) error {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: req.Namespace,
			Name:      req.Name,
			Labels:    labels,
		},
	}
	_, err := ctrl.CreateOrUpdate(ctx, r.Client, service, func() error {
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
		service.Spec.Selector = labels
		service.Spec.Ports = ports

		if err := ctrl.SetControllerReference(application, service, r.Scheme); err != nil {
			return fmt.Errorf("failed to set controller reference to application service: %+w", err)
		}
		return nil
	})
	return err
}

func (r *ApplicationReconciler) reconcileDependencies(ctx context.Context, req ctrl.Request,
	application *meshmanagerv1alpha1.Application, labels map[string]string) error {
	applicationRef := meshmanagerv1alpha1.ApplicationRef{
		Namespace: application.Namespace,
		Name:      application.Name,
	}
	application.Status.MissingDependencies = 0
	for _, dependency := range application.Spec.Dependencies {
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

		dependencyApplication := &meshmanagerv1alpha1.Application{}
		err := r.Get(ctx, dependencyName, dependencyApplication)
		if err != nil {
			if errors.IsNotFound(err) {
				application.Status.MissingDependencies += 1
				continue
			} else {
				return fmt.Errorf("failed to get existing dependency: %+w", err)
			}
		}

		isDependencyAlreadyLinked := false
		for _, dependant := range dependencyApplication.Status.Dependants {
			if dependant.Namespace == applicationRef.Namespace && dependant.Name == applicationRef.Name {
				isDependencyAlreadyLinked = true
			}
		}
		if !isDependencyAlreadyLinked {
			dependencyApplication.Status.Dependants = append(dependencyApplication.Status.Dependants, applicationRef)
		}

		if err := r.Status().Update(ctx, dependencyApplication); err != nil {
			return fmt.Errorf("failed to update dependency application %s/%s: %+w",
				dependencyName.Namespace, dependencyName.Name, err)
		}
	}
	return nil
}

func (r *ApplicationReconciler) finalize(ctx context.Context, req ctrl.Request,
	application *meshmanagerv1alpha1.Application) (bool, error) {
	for _, dependency := range application.Spec.Dependencies {
		dependencyApplication := &meshmanagerv1alpha1.Application{}
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
		err := r.Get(ctx, dependencyName, dependencyApplication)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			} else {
				return false, fmt.Errorf("failed to get existing dependency dependant slice: %+w", err)
			}
		}

		// Removing application from dependant slice of dependency
		dependantIndex := -1
		for index, dependant := range dependencyApplication.Status.Dependants {
			if dependant.Namespace == application.Namespace && dependant.Name == application.Name {
				dependantIndex = index
			}
		}
		if dependantIndex >= 0 {
			dependants := dependencyApplication.Status.Dependants
			dependencyApplication.Status.Dependants = append(dependants[:dependantIndex], dependants[dependantIndex+1:]...)
			if err := r.Status().Update(ctx, dependencyApplication); err != nil {
				return false, fmt.Errorf("failed to remove application from dependants list: %+w", err)
			}
		}
	}
	return len(application.Status.Dependants) == 0, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&meshmanagerv1alpha1.Application{}).
		Complete(r)
}
