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
	"strings"

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
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlController "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	meshmanagerv1alpha1 "github.com/nadundesilva/mesh-manager/api/v1alpha1"
	meshmanagerutils "github.com/nadundesilva/mesh-manager/controllers/utils"
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
				err := meshmanagerutils.UpdateMicroservice(ctx, r.Client, req.NamespacedName,
					func(ms *meshmanagerv1alpha1.Microservice, update meshmanagerutils.UpdateFunc) error {
						controllerutil.RemoveFinalizer(ms, microserviceFinalizer)
						return update()
					},
				)
				if err != nil {
					return ctrl.Result{}, err
				}
			} else {
				return ctrl.Result{}, nil
			}
		}
		return ctrl.Result{}, nil
	}

	// Adding finalizer
	if !controllerutil.ContainsFinalizer(microservice, microserviceFinalizer) {
		err := meshmanagerutils.UpdateMicroservice(ctx, r.Client, req.NamespacedName,
			func(ms *meshmanagerv1alpha1.Microservice, update meshmanagerutils.UpdateFunc) error {
				controllerutil.AddFinalizer(ms, microserviceFinalizer)
				return update()
			},
		)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to add finalizer: %+w", err)
		}
	}

	// Fixing dependencies
	if err := r.reconcileDependencies(ctx, req.NamespacedName); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile microservice dependency slices: %+w", err)
	}

	// Creating sub-resources
	if microservice.Status.MissingDependencies != nil {
		if len(microservice.Spec.Dependencies) == 0 || len(*microservice.Status.MissingDependencies) == 0 {
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
			r.Recorder.Event(microservice, "Warning", FailedDependencyResolutionEvent,
				"Failed to find all dependencies within the cluster")
			logger.Info("Microservice resources not created since it contains missing dependencies",
				"totalDependenciesCount", len(microservice.Spec.Dependencies),
				"missingDependenciesCount", microservice.Status.MissingDependencies)
			return ctrl.Result{}, nil
		}
	}
	return ctrl.Result{}, nil
}

func (r *MicroserviceReconciler) reconcileDeployment(ctx context.Context, req ctrl.Request,
	microservice *meshmanagerv1alpha1.Microservice, parentLabels map[string]string) error {
	deployment := &appsv1.Deployment{}
	result, err := meshmanagerutils.CreateOrUpdate(ctx, r.Client, req.NamespacedName, deployment, func() error {
		deployment.SetLabels(parentLabels)
		deployment.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: parentLabels,
		}
		deployment.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: parentLabels,
			},
			Spec: microservice.Spec.PodSpec,
		}
		deployment.Spec.Replicas = microservice.Spec.Replicas
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
		if *microservice.Spec.Replicas != microservice.Status.Replicas {
			r.Recorder.Eventf(microservice, "Normal", CreatedDeploymentEvent, "Scaled microservice to %d replicas: %s",
				*microservice.Spec.Replicas, deployment.GetName())
		}

		err := meshmanagerutils.UpdateMicroserviceStatus(ctx, r.Client, req.NamespacedName,
			func(ms *meshmanagerv1alpha1.Microservice, update meshmanagerutils.UpdateFunc) error {
				ms.Status.Selector = labels.Set(parentLabels).String()
				ms.Status.Replicas = *ms.Spec.Replicas
				return update()
			},
		)
		if err != nil {
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
				Name: func() string {
					if port.Name == "" {
						return fmt.Sprintf("port-%s-%d", strings.ToLower(string(port.Protocol)), port.ContainerPort)
					}
					return port.Name
				}(),
				Protocol: port.Protocol,
				Port:     port.ContainerPort,
				TargetPort: func() intstr.IntOrString {
					if port.Name == "" {
						return intstr.FromInt(int(port.ContainerPort))
					} else {
						return intstr.FromString(port.Name)
					}
				}(),
			})
		}
	}

	service := &corev1.Service{}
	result, err := meshmanagerutils.CreateOrUpdate(ctx, r.Client, req.NamespacedName, service, func() error {
		service.SetLabels(parentLabels)
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

	netpol := &networkingv1.NetworkPolicy{}
	result, err = meshmanagerutils.CreateOrUpdate(ctx, r.Client, req.NamespacedName, netpol, func() error {
		netpol.SetLabels(parentLabels)
		netpol.Spec.PodSelector = metav1.LabelSelector{
			MatchLabels: parentLabels,
		}
		netpol.Spec.PolicyTypes = []networkingv1.PolicyType{
			networkingv1.PolicyTypeIngress,
		}
		if len(microservice.Status.Dependents) > 0 || len(microservice.Spec.AllowedIngressPeers) > 0 {
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
						peers = append(peers, microservice.Spec.AllowedIngressPeers...)
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

func (r *MicroserviceReconciler) reconcileDependencies(ctx context.Context, name apitypes.NamespacedName) error {
	microserviceRef := meshmanagerv1alpha1.MicroserviceRef{
		Namespace: name.Namespace,
		Name:      name.Name,
	}
	return meshmanagerutils.UpdateMicroserviceStatus(ctx, r.Client, name,
		func(ms *meshmanagerv1alpha1.Microservice, update meshmanagerutils.UpdateFunc) error {
			missingDependencies := []meshmanagerv1alpha1.MicroserviceRef{}
			for _, dependency := range ms.Spec.Dependencies {
				dependencyName := apitypes.NamespacedName{
					Namespace: dependency.Namespace,
					Name:      dependency.Name,
				}
				err := meshmanagerutils.UpdateMicroserviceStatus(ctx, r.Client, dependencyName,
					func(dependencyMs *meshmanagerv1alpha1.Microservice, updateDependency meshmanagerutils.UpdateFunc) error {
						// Adding current microservice to dependency's dependents list
						isDependencyAlreadyLinked := false
						for _, dependent := range dependencyMs.Status.Dependents {
							if dependent.Namespace == microserviceRef.Namespace && dependent.Name == microserviceRef.Name {
								isDependencyAlreadyLinked = true
							}
						}
						if !isDependencyAlreadyLinked {
							dependencyMs.Status.Dependents = append(dependencyMs.Status.Dependents, microserviceRef)
							if err := updateDependency(); err != nil {
								return err
							}
							r.Recorder.Eventf(dependencyMs, "Normal", AddedDependentEvent, "Added dependent %s/%s",
								ms.Namespace, ms.Name)
						}
						return nil
					},
				)
				if err != nil {
					if errors.IsNotFound(err) {
						dependencyRef := meshmanagerv1alpha1.MicroserviceRef{
							Namespace: dependencyName.Namespace,
							Name:      dependencyName.Name,
						}
						missingDependencies = append(missingDependencies, dependencyRef)
						continue
					} else {
						return fmt.Errorf("failed to update dependency microservice %s/%s: %+w",
							dependencyName.Namespace, dependencyName.Name, err)
					}
				}
			}
			ms.Status.MissingDependencies = &missingDependencies
			if err := update(); err != nil {
				return fmt.Errorf("failed to update existing microservice: %+w", err)
			}
			return nil
		},
	)
}

func (r *MicroserviceReconciler) finalize(ctx context.Context, req ctrl.Request,
	microservice *meshmanagerv1alpha1.Microservice) (bool, error) {
	for _, dependency := range microservice.Spec.Dependencies {
		dependencyName := apitypes.NamespacedName{
			Namespace: dependency.Namespace,
			Name:      dependency.Name,
		}
		err := meshmanagerutils.UpdateMicroserviceStatus(ctx, r.Client, dependencyName,
			func(dependencyMs *meshmanagerv1alpha1.Microservice, update meshmanagerutils.UpdateFunc) error {
				// Removing microservice from dependent slice of dependency
				dependentIndex := -1
				for index, dependent := range dependencyMs.Status.Dependents {
					if dependent.Namespace == microservice.Namespace && dependent.Name == microservice.Name {
						dependentIndex = index
					}
				}
				if dependentIndex >= 0 {
					dependents := dependencyMs.Status.Dependents
					dependencyMs.Status.Dependents = append(dependents[:dependentIndex], dependents[dependentIndex+1:]...)
					if err := update(); err != nil {
						return err
					}
					r.Recorder.Eventf(dependencyMs, "Normal", RemovedDependentEvent, "Removed dependent %s/%s",
						microservice.Namespace, microservice.Name)
				}
				return nil
			},
		)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			} else {
				return false, fmt.Errorf("failed to remove microservice from dependents list: %+w", err)
			}
		}
	}
	return len(microservice.Status.Dependents) == 0, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MicroserviceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	options := ctrlController.Options{
		MaxConcurrentReconciles: 100,
		RecoverPanic:            ptr.To(true),
		NeedLeaderElection:      ptr.To(true),
	}
	return ctrl.NewControllerManagedBy(mgr).
		Named("microservice-controller").
		For(&meshmanagerv1alpha1.Microservice{}).
		Owns(&meshmanagerv1alpha1.Microservice{}).
		WithOptions(options).
		Complete(r)
}
