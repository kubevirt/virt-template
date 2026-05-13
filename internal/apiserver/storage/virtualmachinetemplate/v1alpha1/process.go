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
	"encoding/json"
	"fmt"
	"net/http"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/klog/v2"

	"kubevirt.io/virt-template-api/core/subresourcesv1alpha1"
	"kubevirt.io/virt-template-api/core/subresourcesv1beta1"
	templateclient "kubevirt.io/virt-template-client-go/virttemplate"
	"kubevirt.io/virt-template-engine/template"

	"kubevirt.io/virt-template/internal/apiserver/storage/virtualmachinetemplate"
)

type ProcessREST struct {
	client    templateclient.Interface
	processor virtualmachinetemplate.Processor
}

func NewProcessREST(client templateclient.Interface) *ProcessREST {
	return &ProcessREST{
		client:    client,
		processor: template.GetDefaultProcessor(),
	}
}

var (
	_ = rest.Storage(&ProcessREST{})
	_ = rest.Connecter(&ProcessREST{})
)

func (p *ProcessREST) New() runtime.Object {
	return &subresourcesv1alpha1.ProcessedVirtualMachineTemplate{}
}

func (p *ProcessREST) Destroy() {}

func (p *ProcessREST) Connect(ctx context.Context, id string, _ runtime.Object, r rest.Responder) (http.Handler, error) {
	ns, ok := request.NamespaceFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("missing namespace")
	}

	return http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		klog.V(virtualmachinetemplate.DebugLogLevel).Infof("POST /process (v1alpha1) for VirtualMachineTemplate %s/%s", ns, id)

		processed, err := virtualmachinetemplate.ProcessTemplate(ctx, p.client, p.processor, req.Body, ns, id)
		if err != nil {
			r.Error(err)
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

func (p *ProcessREST) NewConnectOptions() (options runtime.Object, include bool, path string) {
	return nil, false, ""
}

func (p *ProcessREST) ConnectMethods() []string {
	return []string{http.MethodPost}
}

func convertProcessedToV1alpha1(in *subresourcesv1beta1.ProcessedVirtualMachineTemplate) (*subresourcesv1alpha1.ProcessedVirtualMachineTemplate, error) {
	data, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	out := &subresourcesv1alpha1.ProcessedVirtualMachineTemplate{}
	return out, json.Unmarshal(data, out)
}
