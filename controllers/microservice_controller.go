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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	meshmanagerv1alpha1 "github.com/nadundesilva/mesh-manager/api/v1alpha1"
)

const (
	groupFqn              = "mesh-manager.nadundesilva.github.io"
	microserviceFinalizer = groupFqn + "/finalizer"
	microserviceLabel     = groupFqn + "/microservice"

	RemovedDependentEvent            = "RemovedDependent"
	AddedDependentEvent              = "AddedDependent"
	FailedDependencyResolutionEvent  = "FailedDependencyResolution"
	CreatedDeploymentEvent           = "CreatedDeployment"
	UpdatedDeploymentEvent           = "UpdatedDeployment"
	FailedUpdatingDeploymentEvent    = "FailedUpdatingDeployment"
	CreatedServiceEvent              = "CreatedService"
	UpdatedServiceEvent              = "UpdatedService"
	FailedUpdatingServiceEvent       = "FailedUpdatingService"
	CreatedNetworkPolicyEvent        = "CreatedNetworkPolicy"
	UpdatedNetworkPolicyEvent        = "UpdatedNetworkPolicy"
	FailedUpdatingNetworkPolicyEvent = "FailedUpdatingNetworkPolicy"
)

// MicroserviceReconciler reconciles a Microservice object
type MicroserviceReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=mesh-manager.nadundesilva.github.io,resources=microservices,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mesh-manager.nadundesilva.github.io,resources=microservices/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mesh-manager.nadundesilva.github.io,resources=microservices/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Microservice object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *MicroserviceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("microservice", req.NamespacedName)
	ctx = log.IntoContext(ctx, logger)
	logger.Info("Reconciling microservice")

	microservice := &meshmanagerv1alpha1.Microservice{}
	if err := r.Get(ctx, req.NamespacedName, microservice); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		} else {
			return ctrl.Result{}, fmt.Errorf("failed to get existing microservice: %+w", err)
		}
	}

	// Finalizing if required
	isMicroserviceMarkedToBeDeleted := microservice.GetDeletionTimestamp() != nil
	if isMicroserviceMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(microservice, microserviceFinalizer) {
			isFinalized, err := r.finalize(ctx, req, microservice)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to finalize microservice: %+w", err)
			}

			if isFinalized {
				controllerutil.RemoveFinalizer(microservice, microserviceFinalizer)
				err := r.Update(ctx, microservice)
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
	if !controllerutil.ContainsFinalizer(microservice, microserviceFinalizer) {
		controllerutil.AddFinalizer(microservice, microserviceFinalizer)
		err := r.Update(ctx, microservice)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to add finalizer: %+w", err)
		}
	}

	// Fixing dependencies
	if err := r.reconcileDependencies(ctx, req, microservice); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile microservice dependency slices: %+w", err)
	}
	if err := r.Status().Update(ctx, microservice); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update existing microservice: %+w", err)
	}

	// Creating sub-resources
	if len(microservice.Spec.Dependencies) == 0 || len(microservice.Status.MissingDependencies) == 0 {
		parentLabels := map[string]string{
			microserviceLabel: req.Name,
		}
		for k, v := range microservice.Labels {
			parentLabels[k] = v
		}

		if err := r.reconcileDeployment(ctx, req, microservice, parentLabels); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to reconcile microservice deployment: %+w", err)
		}
		if err := r.reconcileNetworking(ctx, req, microservice, parentLabels); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to reconcile microservice service: %+w", err)
		}
		return ctrl.Result{}, nil
	} else {
		r.Recorder.Eventf(microservice, "Warning", FailedDependencyResolutionEvent,
			"Failed to find all dependencies within the cluster")
		logger.Info("Microservice resources not created since it contains missing dependencies",
			"totalDependenciesCount", len(microservice.Spec.Dependencies),
			"missingDependenciesCount", microservice.Status.MissingDependencies)
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}
}

func (r *MicroserviceReconciler) reconcileDeployment(ctx context.Context, req ctrl.Request,
	microservice *meshmanagerv1alpha1.Microservice, parentLabels map[string]string) error {
	var newReplicaCount *int32
	if microservice.Spec.Replicas != nil {
		newReplicaCount = microservice.Spec.Replicas
	} else {
		newReplicaCount = pointer.Int32(1)
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: req.Namespace,
			Name:      req.Name,
			Labels:    parentLabels,
		},
	}
	result, err := ctrl.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		deployment.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: parentLabels,
		}
		deployment.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: parentLabels,
			},
			Spec: microservice.Spec.PodSpec,
		}
		deployment.Spec.Replicas = newReplicaCount
		if err := ctrl.SetControllerReference(microservice, deployment, r.Scheme); err != nil {
			return fmt.Errorf("failed to set controller reference to microservice deployment: %+w", err)
		}
		return nil
	})
	if err != nil {
		r.Recorder.Eventf(microservice, "Warning", FailedUpdatingDeploymentEvent,
			"Failed creating/updating deployment: %v", err)
		return err
	} else {
		if *newReplicaCount != microservice.Status.Replicas {
			r.Recorder.Eventf(microservice, "Normal", CreatedDeploymentEvent, "Scaled microservice to %d replicas: %s",
				*newReplicaCount, deployment.GetName())
		}

		microservice.Status.Selector = labels.Set(parentLabels).String()
		microservice.Status.Replicas = *newReplicaCount
		if err := r.Status().Update(ctx, microservice); err != nil {
			return fmt.Errorf("failed to update status with replica count: %+w", err)
		}
	}
	if result == controllerutil.OperationResultCreated {
		r.Recorder.Eventf(microservice, "Normal", CreatedDeploymentEvent, "Created deployment: %s",
			deployment.GetName())
	} else if result == controllerutil.OperationResultUpdated {
		r.Recorder.Eventf(microservice, "Normal", UpdatedDeploymentEvent, "Updated deployment: %s",
			deployment.GetName())
	}
	return nil
}

func (r *MicroserviceReconciler) reconcileNetworking(ctx context.Context, req ctrl.Request,
	microservice *meshmanagerv1alpha1.Microservice, parentLabels map[string]string) error {
	ports := []corev1.ServicePort{}
	for _, container := range microservice.Spec.PodSpec.Containers {
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
			Labels:    parentLabels,
		},
	}
	result, err := ctrl.CreateOrUpdate(ctx, r.Client, service, func() error {
		service.Spec.Selector = parentLabels
		service.Spec.Ports = ports

		if err := ctrl.SetControllerReference(microservice, service, r.Scheme); err != nil {
			return fmt.Errorf("failed to set controller reference to microservice service: %+w", err)
		}
		return nil
	})
	if err != nil {
		r.Recorder.Eventf(microservice, "Warning", FailedUpdatingServiceEvent,
			"Failed creating/updating service: %v", err)
		return err
	}
	if result == controllerutil.OperationResultCreated {
		r.Recorder.Eventf(microservice, "Normal", CreatedServiceEvent, "Created service: %s",
			service.GetName())
	} else if result == controllerutil.OperationResultUpdated {
		r.Recorder.Eventf(microservice, "Normal", UpdatedServiceEvent, "Updated service: %s",
			service.GetName())
	}

	netpol := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: req.Namespace,
			Name:      req.Name,
			Labels:    parentLabels,
		},
	}
	result, err = ctrl.CreateOrUpdate(ctx, r.Client, netpol, func() error {
		netpol.Spec.PodSelector = metav1.LabelSelector{
			MatchLabels: parentLabels,
		}
		netpol.Spec.PolicyTypes = []networkingv1.PolicyType{
			networkingv1.PolicyTypeIngress,
		}
		if len(microservice.Status.Dependents) > 0 {
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
						for _, dependent := range microservice.Status.Dependents {
							peers = append(peers, networkingv1.NetworkPolicyPeer{
								NamespaceSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"kubernetes.io/metadata.name": dependent.Namespace,
									},
								},
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										microserviceLabel: dependent.Name,
									},
								},
							})
						}
						return peers
					}(),
				},
			}
		}

		if err := ctrl.SetControllerReference(microservice, netpol, r.Scheme); err != nil {
			return fmt.Errorf("failed to set controller reference to microservice network policy: %+w", err)
		}
		return nil
	})
	if err != nil {
		r.Recorder.Eventf(microservice, "Warning", FailedUpdatingNetworkPolicyEvent,
			"Failed creating/updating network policy: %v", err)
		return err
	}
	if result == controllerutil.OperationResultCreated {
		r.Recorder.Eventf(microservice, "Normal", CreatedNetworkPolicyEvent, "Created network policy: %s",
			netpol.GetName())
	} else if result == controllerutil.OperationResultUpdated {
		r.Recorder.Eventf(microservice, "Normal", UpdatedNetworkPolicyEvent, "Updated network policy: %s",
			netpol.GetName())
	}
	return nil
}

func (r *MicroserviceReconciler) reconcileDependencies(ctx context.Context, req ctrl.Request,
	microservice *meshmanagerv1alpha1.Microservice) error {
	microserviceRef := meshmanagerv1alpha1.MicroserviceRef{
		Namespace: microservice.Namespace,
		Name:      microservice.Name,
	}
	microservice.Status.MissingDependencies = []meshmanagerv1alpha1.MicroserviceRef{}
	for _, dependency := range microservice.Spec.Dependencies {
		dependencyName := apitypes.NamespacedName{
			Namespace: func() string {
				if dependency.Namespace == "" {
					return microservice.GetNamespace()
				} else {
					return dependency.Namespace
				}
			}(),
			Name: dependency.Name,
		}

		dependencyMicroservice := &meshmanagerv1alpha1.Microservice{}
		err := r.Get(ctx, dependencyName, dependencyMicroservice)
		if err != nil {
			if errors.IsNotFound(err) {
				dependencyRef := meshmanagerv1alpha1.MicroserviceRef{
					Namespace: dependencyName.Namespace,
					Name:      dependencyName.Name,
				}
				microservice.Status.MissingDependencies = append(microservice.Status.MissingDependencies,
					dependencyRef)
				continue
			} else {
				return fmt.Errorf("failed to get existing dependency: %+w", err)
			}
		}

		// Adding current microservice to dependency's dependents list
		isDependencyAlreadyLinked := false
		for _, dependent := range dependencyMicroservice.Status.Dependents {
			if dependent.Namespace == microserviceRef.Namespace && dependent.Name == microserviceRef.Name {
				isDependencyAlreadyLinked = true
			}
		}
		if !isDependencyAlreadyLinked {
			dependencyMicroservice.Status.Dependents = append(dependencyMicroservice.Status.Dependents, microserviceRef)
			if err := r.Status().Update(ctx, dependencyMicroservice); err != nil {
				return fmt.Errorf("failed to update dependency microservice %s/%s: %+w",
					dependencyName.Namespace, dependencyName.Name, err)
			}
			r.Recorder.Eventf(dependencyMicroservice, "Normal", AddedDependentEvent, "Added dependent %s/%s",
				microservice.Namespace, microservice.Name)
		}
	}
	return nil
}

func (r *MicroserviceReconciler) finalize(ctx context.Context, req ctrl.Request,
	microservice *meshmanagerv1alpha1.Microservice) (bool, error) {
	for _, dependency := range microservice.Spec.Dependencies {
		dependencyMicroservice := &meshmanagerv1alpha1.Microservice{}
		dependencyName := apitypes.NamespacedName{
			Namespace: func() string {
				if dependency.Namespace == "" {
					return microservice.GetNamespace()
				} else {
					return dependency.Namespace
				}
			}(),
			Name: dependency.Name,
		}
		err := r.Get(ctx, dependencyName, dependencyMicroservice)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			} else {
				return false, fmt.Errorf("failed to get existing dependency dependent slice: %+w", err)
			}
		}

		// Removing microservice from dependent slice of dependency
		dependentIndex := -1
		for index, dependent := range dependencyMicroservice.Status.Dependents {
			if dependent.Namespace == microservice.Namespace && dependent.Name == microservice.Name {
				dependentIndex = index
			}
		}
		if dependentIndex >= 0 {
			dependents := dependencyMicroservice.Status.Dependents
			dependencyMicroservice.Status.Dependents = append(dependents[:dependentIndex], dependents[dependentIndex+1:]...)
			if err := r.Status().Update(ctx, dependencyMicroservice); err != nil {
				return false, fmt.Errorf("failed to remove microservice from dependents list: %+w", err)
			}
			r.Recorder.Eventf(dependencyMicroservice, "Normal", RemovedDependentEvent, "Removed dependent %s/%s",
				microservice.Namespace, microservice.Name)
		}
	}
	return len(microservice.Status.Dependents) == 0, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MicroserviceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&meshmanagerv1alpha1.Microservice{}).
		Complete(r)
}
