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

var _ = Describe("Calling webhook", func() {
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
	defaultingTestEntries := []TableEntry{
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
		defaultingTestEntries = append(defaultingTestEntries,
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

	defaultingTestEntries = append(defaultingTestEntries,
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

	DescribeTable("Defaults missing values",
		func(object *Microservice, expectedObject *Microservice) {
			object.Default()
			Expect(object).To(Equal(expectedObject))
		},
		defaultingTestEntries...,
	)

	DescribeTable("Validates create and update new values",
		func(object *Microservice, expectedError error) {
			err := object.ValidateCreate()
			if expectedError == nil {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(MatchError(expectedError))
			}

			oldObject := object.DeepCopy()
			oldObject.Spec.Replicas = pointer.Int32(*oldObject.Spec.Replicas + 1)
			oldObject.Spec.Dependencies = []MicroserviceRef{}

			err = object.ValidateUpdate(oldObject)
			if expectedError == nil {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(MatchError(expectedError))
			}

		},
		Entry("When replica count is below 0",
			&Microservice{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace-1",
					Name:      "microservice-1",
				},
				Spec: MicroserviceSpec{
					Replicas: pointer.Int32(-1),
				},
			},
			fmt.Errorf("microservice replica count cannot be below zero"),
		),
		Entry("When replica count is 0",
			&Microservice{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace-2",
					Name:      "microservice-2",
				},
				Spec: MicroserviceSpec{
					Replicas: pointer.Int32(0),
				},
			},
			nil,
		),
		Entry("When replica count is above 0",
			&Microservice{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace-3",
					Name:      "microservice-3",
				},
				Spec: MicroserviceSpec{
					Replicas: pointer.Int32(1),
				},
			},
			nil,
		),
		Entry("When a single port is duplicated in a single container",
			&Microservice{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace-4",
					Name:      "microservice-4",
				},
				Spec: MicroserviceSpec{
					Replicas: pointer.Int32(1),
					PodSpec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Ports: []corev1.ContainerPort{
									{
										ContainerPort: 80,
									},
									{
										ContainerPort: 8080,
									},
									{
										ContainerPort: 80,
									},
								},
							},
						},
					},
				},
			},
			fmt.Errorf("microservice cannot contain duplicated port(s) [80]"),
		),
		Entry("When two ports are duplicated in a single container",
			&Microservice{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace-4",
					Name:      "microservice-4",
				},
				Spec: MicroserviceSpec{
					Replicas: pointer.Int32(1),
					PodSpec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Ports: []corev1.ContainerPort{
									{
										ContainerPort: 80,
									},
									{
										ContainerPort: 8080,
									},
									{
										ContainerPort: 80,
									},
									{
										ContainerPort: 90,
									},
									{
										ContainerPort: 100,
									},
									{
										ContainerPort: 8080,
									},
								},
							},
						},
					},
				},
			},
			fmt.Errorf("microservice cannot contain duplicated port(s) [80 8080]"),
		),
		Entry("When a two ports are duplicated across two containers",
			&Microservice{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace-4",
					Name:      "microservice-4",
				},
				Spec: MicroserviceSpec{
					Replicas: pointer.Int32(1),
					PodSpec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Ports: []corev1.ContainerPort{
									{
										ContainerPort: 80,
									},
									{
										ContainerPort: 8080,
									},
									{
										ContainerPort: 9080,
									},
								},
							},
							{
								Ports: []corev1.ContainerPort{
									{
										ContainerPort: 1070,
									},
									{
										ContainerPort: 8070,
									},
									{
										ContainerPort: 9070,
									},
									{
										ContainerPort: 9071,
									},
									{
										ContainerPort: 9072,
									},
								},
							},
							{
								Ports: []corev1.ContainerPort{
									{
										ContainerPort: 7080,
									},
									{
										ContainerPort: 8080,
									},
									{
										ContainerPort: 3080,
									},
									{
										ContainerPort: 1070,
									},
								},
							},
						},
					},
				},
			},
			fmt.Errorf("microservice cannot contain duplicated port(s) [8080 1070]"),
		),
	)
})
