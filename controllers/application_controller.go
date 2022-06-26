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
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	meshmanagerv1alpha1 "github.com/nadundesilva/mesh-manager/api/v1alpha1"
)

const (
	groupFqn             = "mesh-manager.nadundesilva.github.io"
	applicationFinalizer = groupFqn + "/finalizer"
	applicationLabel     = groupFqn + "/application"

	RemovedDependentEvent = "RemovedDependent"
	AddedDependentEvent   = "AddedDependent"
)

// ApplicationReconciler reconciles a Application object
type ApplicationReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=mesh-manager.nadundesilva.github.io,resources=applications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mesh-manager.nadundesilva.github.io,resources=applications/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mesh-manager.nadundesilva.github.io,resources=applications/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

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

	// Finalizing if required
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

	// Adding finalizer
	if !controllerutil.ContainsFinalizer(application, applicationFinalizer) {
		controllerutil.AddFinalizer(application, applicationFinalizer)
		err := r.Update(ctx, application)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to add finalizer: %+w", err)
		}
	}

	// Fixing dependencies
	if err := r.reconcileDependencies(ctx, req, application); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile application dependency slices: %+w", err)
	}
	if err := r.Status().Update(ctx, application); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update existing application: %+w", err)
	}

	// Creating sub-resources
	labels := map[string]string{
		applicationLabel: req.Name,
	}
	for k, v := range application.Labels {
		labels[k] = v
	}
	if len(application.Spec.Dependencies) == 0 || application.Status.MissingDependencies == 0 {
		if err := r.reconcileDeployment(ctx, req, application, labels); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to reconcile application deployment: %+w", err)
		}
		if err := r.reconcileNetworking(ctx, req, application, labels); err != nil {
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

func (r *ApplicationReconciler) reconcileNetworking(ctx context.Context, req ctrl.Request,
	application *meshmanagerv1alpha1.Application, labels map[string]string) error {
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

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: req.Namespace,
			Name:      req.Name,
			Labels:    labels,
		},
	}
	_, err := ctrl.CreateOrUpdate(ctx, r.Client, service, func() error {
		service.Spec.Selector = labels
		service.Spec.Ports = ports

		if err := ctrl.SetControllerReference(application, service, r.Scheme); err != nil {
			return fmt.Errorf("failed to set controller reference to application service: %+w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	netpol := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: req.Namespace,
			Name:      req.Name,
			Labels:    labels,
		},
	}
	_, err = ctrl.CreateOrUpdate(ctx, r.Client, netpol, func() error {
		netpol.Spec.PodSelector = metav1.LabelSelector{
			MatchLabels: labels,
		}
		netpol.Spec.PolicyTypes = []networkingv1.PolicyType{
			networkingv1.PolicyTypeIngress,
		}
		if len(application.Status.Dependents) > 0 {
			netpol.Spec.Ingress = []networkingv1.NetworkPolicyIngressRule{
				{
					Ports: func() []networkingv1.NetworkPolicyPort {
						netpolPorts := []networkingv1.NetworkPolicyPort{}
						for _, port := range ports {
							netpolPorts = append(netpolPorts, networkingv1.NetworkPolicyPort{
								Protocol: &port.Protocol,
								Port:     &port.TargetPort,
							})
						}
						return netpolPorts
					}(),
					From: func() []networkingv1.NetworkPolicyPeer {
						peers := []networkingv1.NetworkPolicyPeer{}
						for _, dependent := range application.Status.Dependents {
							peers = append(peers, networkingv1.NetworkPolicyPeer{
								NamespaceSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"kubernetes.io/metadata.name": dependent.Namespace,
									},
								},
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										applicationLabel: dependent.Name,
									},
								},
							})
						}
						return peers
					}(),
				},
			}
		}

		if err := ctrl.SetControllerReference(application, netpol, r.Scheme); err != nil {
			return fmt.Errorf("failed to set controller reference to application network policy: %+w", err)
		}
		return nil
	})
	return err
}

func (r *ApplicationReconciler) reconcileDependencies(ctx context.Context, req ctrl.Request,
	application *meshmanagerv1alpha1.Application) error {
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

		// Adding current application to dependency's dependents list
		isDependencyAlreadyLinked := false
		for _, dependent := range dependencyApplication.Status.Dependents {
			if dependent.Namespace == applicationRef.Namespace && dependent.Name == applicationRef.Name {
				isDependencyAlreadyLinked = true
			}
		}
		if !isDependencyAlreadyLinked {
			dependencyApplication.Status.Dependents = append(dependencyApplication.Status.Dependents, applicationRef)
			if err := r.Status().Update(ctx, dependencyApplication); err != nil {
				return fmt.Errorf("failed to update dependency application %s/%s: %+w",
					dependencyName.Namespace, dependencyName.Name, err)
			}
			r.Recorder.Eventf(dependencyApplication, "Normal", AddedDependentEvent, "Added dependent %s/%s",
				application.Namespace, application.Name)
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
				return false, fmt.Errorf("failed to get existing dependency dependent slice: %+w", err)
			}
		}

		// Removing application from dependent slice of dependency
		dependentIndex := -1
		for index, dependent := range dependencyApplication.Status.Dependents {
			if dependent.Namespace == application.Namespace && dependent.Name == application.Name {
				dependentIndex = index
			}
		}
		if dependentIndex >= 0 {
			dependents := dependencyApplication.Status.Dependents
			dependencyApplication.Status.Dependents = append(dependents[:dependentIndex], dependents[dependentIndex+1:]...)
			if err := r.Status().Update(ctx, dependencyApplication); err != nil {
				return false, fmt.Errorf("failed to remove application from dependents list: %+w", err)
			}
			r.Recorder.Eventf(dependencyApplication, "Normal", RemovedDependentEvent, "Removed dependent %s/%s",
				application.Namespace, application.Name)
		}
	}
	return len(application.Status.Dependents) == 0, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&meshmanagerv1alpha1.Application{}).
		Complete(r)
}
