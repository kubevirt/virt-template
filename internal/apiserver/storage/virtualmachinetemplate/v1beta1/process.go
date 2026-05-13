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

package v1beta1

import (
	"context"
	"fmt"
	"net/http"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/klog/v2"

	"kubevirt.io/virt-template-api/core/subresourcesv1beta1"
	templateclient "kubevirt.io/virt-template-client-go/virttemplate"
	"kubevirt.io/virt-template-engine/template"

	"kubevirt.io/virt-template/internal/apiserver/storage/virtualmachinetemplate"
)

type V1beta1ProcessREST struct {
	client    templateclient.Interface
	processor virtualmachinetemplate.Processor
}

func NewV1beta1ProcessREST(client templateclient.Interface) *V1beta1ProcessREST {
	return &V1beta1ProcessREST{
		client:    client,
		processor: template.GetDefaultProcessor(),
	}
}

var (
	_ = rest.Storage(&V1beta1ProcessREST{})
	_ = rest.Connecter(&V1beta1ProcessREST{})
)

func (p *V1beta1ProcessREST) New() runtime.Object {
	return &subresourcesv1beta1.ProcessedVirtualMachineTemplate{}
}

func (p *V1beta1ProcessREST) Destroy() {}

func (p *V1beta1ProcessREST) Connect(ctx context.Context, id string, _ runtime.Object, r rest.Responder) (http.Handler, error) {
	ns, ok := request.NamespaceFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("missing namespace")
	}

	return http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		klog.V(virtualmachinetemplate.DebugLogLevel).Infof("POST /process for VirtualMachineTemplate %s/%s", ns, id)

		processed, err := virtualmachinetemplate.ProcessTemplate(ctx, p.client, p.processor, req.Body, ns, id)
		if err != nil {
			r.Error(err)
			return
		}

		r.Object(http.StatusOK, processed)
	}), nil
}

func (p *V1beta1ProcessREST) NewConnectOptions() (options runtime.Object, include bool, path string) {
	return nil, false, ""
}

func (p *V1beta1ProcessREST) ConnectMethods() []string {
	return []string{http.MethodPost}
}
