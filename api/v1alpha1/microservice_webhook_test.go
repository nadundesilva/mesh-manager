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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

var _ = Describe("Webhook functionalities", func() {
	completeMicroservice := &Microservice{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "namespace-0",
			Name:      "microservice-0",
		},
		Spec: MicroserviceSpec{
			Replicas: pointer.Int32(2),
			PodSpec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "test-container",
						Image: "test-registry.com/test-image:latest",
					},
				},
			},
			Dependencies: []MicroserviceRef{
				{
					Namespace: "namespace-1",
					Name:      "dependency-1",
				},
				{
					Namespace: "namespace-2",
					Name:      "dependency-2",
				},
			},
		},
	}
	entries := []TableEntry{
		Entry("When no defaults are required",
			completeMicroservice,
			completeMicroservice,
		),
		Entry("When replicas field is not specified",
			&Microservice{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace-1",
					Name:      "microservice-1",
				},
				Spec: MicroserviceSpec{
					PodSpec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "test-registry.com/test-image:latest",
							},
						},
					},
					Dependencies: []MicroserviceRef{},
				},
			},
			&Microservice{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace-1",
					Name:      "microservice-1",
				},
				Spec: MicroserviceSpec{
					Replicas: pointer.Int32(1),
					PodSpec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "test-registry.com/test-image:latest",
							},
						},
					},
					Dependencies: []MicroserviceRef{},
				},
			},
		),
	}

	replicaCounts := []int32{0, 1, 2}
	for _, replicaCount := range replicaCounts {
		entries = append(entries,
			Entry(fmt.Sprintf("When %d replicas are specified", replicaCount),
				&Microservice{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "namespace-2",
						Name:      fmt.Sprintf("microservice-2-%d", replicaCount),
					},
					Spec: MicroserviceSpec{
						Replicas: pointer.Int32(replicaCount),
						PodSpec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test-container",
									Image: "test-registry.com/test-image:latest",
								},
							},
						},
						Dependencies: []MicroserviceRef{},
					},
				},
				&Microservice{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "namespace-2",
						Name:      fmt.Sprintf("microservice-2-%d", replicaCount),
					},
					Spec: MicroserviceSpec{
						Replicas: pointer.Int32(replicaCount),
						PodSpec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test-container",
									Image: "test-registry.com/test-image:latest",
								},
							},
						},
						Dependencies: []MicroserviceRef{},
					},
				},
			))
	}

	entries = append(entries,
		Entry("When one dependency's namespace field is not specified",
			&Microservice{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace-3",
					Name:      "microservice-3",
				},
				Spec: MicroserviceSpec{
					PodSpec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "test-registry.com/test-image:latest",
							},
						},
					},
					Dependencies: []MicroserviceRef{
						{
							Namespace: "namespace-1",
							Name:      "dependency-1",
						},
						{
							Name: "dependency-2",
						},
						{
							Namespace: "namespace-2",
							Name:      "dependency-3",
						},
					},
				},
			},
			&Microservice{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace-3",
					Name:      "microservice-3",
				},
				Spec: MicroserviceSpec{
					Replicas: pointer.Int32(1),
					PodSpec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "test-registry.com/test-image:latest",
							},
						},
					},
					Dependencies: []MicroserviceRef{
						{
							Namespace: "namespace-1",
							Name:      "dependency-1",
						},
						{
							Namespace: "namespace-3",
							Name:      "dependency-2",
						},
						{
							Namespace: "namespace-2",
							Name:      "dependency-3",
						},
					},
				},
			},
		),
		Entry("When all dependency's namespace field is not specified",
			&Microservice{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace-4",
					Name:      "microservice-4",
				},
				Spec: MicroserviceSpec{
					Replicas: pointer.Int32(2),
					PodSpec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "test-registry.com/test-image:latest",
							},
						},
					},
					Dependencies: []MicroserviceRef{
						{
							Name: "dependency-1",
						},
						{
							Name: "dependency-2",
						},
					},
				},
			},
			&Microservice{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace-4",
					Name:      "microservice-4",
				},
				Spec: MicroserviceSpec{
					Replicas: pointer.Int32(2),
					PodSpec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "test-registry.com/test-image:latest",
							},
						},
					},
					Dependencies: []MicroserviceRef{
						{
							Namespace: "namespace-4",
							Name:      "dependency-1",
						},
						{
							Namespace: "namespace-4",
							Name:      "dependency-2",
						},
					},
				},
			},
		),
		Entry("When all dependency's namespace field is specified",
			&Microservice{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace-5",
					Name:      "microservice-5",
				},
				Spec: MicroserviceSpec{
					Replicas: pointer.Int32(2),
					PodSpec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "test-registry.com/test-image:latest",
							},
						},
					},
					Dependencies: []MicroserviceRef{
						{
							Namespace: "namespace-1",
							Name:      "dependency-1",
						},
						{
							Namespace: "namespace-2",
							Name:      "dependency-2",
						},
					},
				},
			},
			&Microservice{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace-5",
					Name:      "microservice-5",
				},
				Spec: MicroserviceSpec{
					Replicas: pointer.Int32(2),
					PodSpec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "test-registry.com/test-image:latest",
							},
						},
					},
					Dependencies: []MicroserviceRef{
						{
							Namespace: "namespace-1",
							Name:      "dependency-1",
						},
						{
							Namespace: "namespace-2",
							Name:      "dependency-2",
						},
					},
				},
			},
		),
	)

	DescribeTable("Defaulting microservice values",
		func(object *Microservice, expectedObject *Microservice) {
			object.Default()
			Expect(object).To(Equal(expectedObject))
		},
		entries...,
	)
})
