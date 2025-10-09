/*
 * This file is part of the KubeVirt project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright The KubeVirt Authors.
 *
 */

package scheme

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	virtclientgoscheme "kubevirt.io/client-go/kubevirt/scheme"

	templatesubresourcesv1alpha1 "kubevirt.io/virt-template/api/subresourcesv1alpha1"
	templatev1alpha1 "kubevirt.io/virt-template/api/v1alpha1"
)

func New() *runtime.Scheme {
	scheme := runtime.NewScheme()

	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(virtclientgoscheme.AddToScheme(scheme))

	utilruntime.Must(templatesubresourcesv1alpha1.AddToScheme(scheme))
	utilruntime.Must(templatev1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme

	return scheme
}
