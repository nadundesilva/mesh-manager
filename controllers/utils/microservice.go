/*
 * Copyright (c) 2023, Nadun De Silva. All Rights Reserved.
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

package utils

import (
	"context"

	meshmanagerv1alpha1 "github.com/nadundesilva/mesh-manager/api/v1alpha1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type UpdateFunc func() error
type updateObjectFunc func(context.Context, *meshmanagerv1alpha1.Microservice) error
type mutateMicroserviceObjectFunc func(*meshmanagerv1alpha1.Microservice, UpdateFunc) error

func UpdateMicroservice(ctx context.Context, c client.Client, name apitypes.NamespacedName,
	mutate mutateMicroserviceObjectFunc) error {
	return updateMicroserviceWithUpdateFunc(ctx, c, name, mutate,
		func(ctx context.Context, ms *meshmanagerv1alpha1.Microservice) error {
			return c.Update(ctx, ms)
		})
}

func UpdateMicroserviceStatus(ctx context.Context, c client.Client, name apitypes.NamespacedName,
	mutate mutateMicroserviceObjectFunc) error {
	return updateMicroserviceWithUpdateFunc(ctx, c, name, mutate,
		func(ctx context.Context, ms *meshmanagerv1alpha1.Microservice) error {
			return c.Status().Update(ctx, ms)
		})
}

func updateMicroserviceWithUpdateFunc(ctx context.Context, reader client.Reader, name apitypes.NamespacedName,
	mutate mutateMicroserviceObjectFunc, update updateObjectFunc) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		microservice := &meshmanagerv1alpha1.Microservice{}
		if err := reader.Get(ctx, name, microservice); err != nil {
			return err
		}

		return mutate(microservice, func() error {
			return update(ctx, microservice)
		})
	})
}
