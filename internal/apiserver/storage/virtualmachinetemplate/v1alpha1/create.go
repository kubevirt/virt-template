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

package v1alpha1

import (
	"context"
	"fmt"
	"net/http"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/klog/v2"

	"kubevirt.io/client-go/kubecli"

	"kubevirt.io/virt-template-api/core/subresourcesv1alpha1"
	templateclient "kubevirt.io/virt-template-client-go/virttemplate"
	"kubevirt.io/virt-template-engine/template"

	"kubevirt.io/virt-template/internal/apiserver/storage/virtualmachinetemplate"
)

type V1alpha1CreateREST struct {
	client     templateclient.Interface
	virtClient kubecli.KubevirtClient
	processor  virtualmachinetemplate.Processor
}

func NewV1alpha1CreateREST(
	client templateclient.Interface,
	virtClient kubecli.KubevirtClient,
) *V1alpha1CreateREST {
	return &V1alpha1CreateREST{
		client:     client,
		virtClient: virtClient,
		processor:  template.GetDefaultProcessor(),
	}
}

var (
	_ = rest.Storage(&V1alpha1CreateREST{})
	_ = rest.Connecter(&V1alpha1CreateREST{})
)

func (c *V1alpha1CreateREST) New() runtime.Object {
	return &subresourcesv1alpha1.ProcessedVirtualMachineTemplate{}
}

func (c *V1alpha1CreateREST) Destroy() {}

func (c *V1alpha1CreateREST) Connect(ctx context.Context, id string, _ runtime.Object, r rest.Responder) (http.Handler, error) {
	ns, ok := request.NamespaceFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("missing namespace")
	}

	return http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		klog.V(virtualmachinetemplate.DebugLogLevel).Infof("POST /create (v1alpha1) for VirtualMachineTemplate %s/%s", ns, id)

		processed, err := virtualmachinetemplate.ProcessTemplate(ctx, c.client, c.processor, req.Body, ns, id)
		if err != nil {
			r.Error(err)
			return
		}

		processed.VirtualMachine, err = c.virtClient.VirtualMachine(ns).Create(ctx, processed.VirtualMachine, metav1.CreateOptions{})
		if err != nil {
			r.Error(apierrors.NewInternalError(fmt.Errorf("error creating VirtualMachine: %w", err)))
			return
		}

		converted, err := convertProcessedToV1alpha1(processed)
		if err != nil {
			r.Error(err)
			return
		}

		r.Object(http.StatusOK, converted)
	}), nil
}

func (c *V1alpha1CreateREST) NewConnectOptions() (options runtime.Object, include bool, path string) {
	return nil, false, ""
}

func (c *V1alpha1CreateREST) ConnectMethods() []string {
	return []string{http.MethodPost}
}
