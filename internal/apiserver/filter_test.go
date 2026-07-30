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

package apiserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/emicklei/go-restful/v3"
	apidiscoveryv2 "k8s.io/api/apidiscovery/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/endpoints"
	discoveryendpoint "k8s.io/apiserver/pkg/endpoints/discovery/aggregated"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
)

var _ = Describe("Filter", func() {
	const (
		v1alpha1                        = "v1alpha1"
		v1                              = "v1"
		create                          = "create"
		virtualmachines                 = "virtualmachines"
		virtualmachinetemplates         = "virtualmachinetemplates"
		virtualmachinetemplaterequests  = "virtualmachinetemplaterequests"
		processedVirtualMachineTemplate = "ProcessedVirtualMachineTemplate"
		virtualmachinesProcess          = virtualmachines + "/process"
		virtualmachinesCreate           = virtualmachines + "/" + create
		virtualmachinetemplatesStatus   = virtualmachinetemplates + "/status"
		virtualmachinetemplatesProcess  = virtualmachinetemplates + "/process"
		virtualmachinetemplatesCreate   = virtualmachinetemplates + "/" + create
	)

	Context("getParentResourceNames", func() {
		It("should return empty slice for empty storage map", func() {
			resourcesStorage := map[string]rest.Storage{}
			parents := getParentResourceNames(resourcesStorage)
			Expect(parents).To(BeEmpty())
		})

		It("should return empty slice when no subresources exist", func() {
			resourcesStorage := map[string]rest.Storage{
				virtualmachines:         nil,
				virtualmachinetemplates: nil,
			}
			parents := getParentResourceNames(resourcesStorage)
			Expect(parents).To(BeEmpty())
		})

		It("should extract parent resource from single subresource", func() {
			resourcesStorage := map[string]rest.Storage{
				virtualmachines:           nil,
				"virtualmachines/process": nil,
				virtualmachinetemplates:   nil,
			}
			parents := getParentResourceNames(resourcesStorage)
			Expect(parents).To(ConsistOf(virtualmachines))
		})

		It("should extract parent resources from multiple subresources", func() {
			resourcesStorage := map[string]rest.Storage{
				virtualmachines:                  nil,
				"virtualmachines/process":        nil,
				"virtualmachines/create":         nil,
				virtualmachinetemplates:          nil,
				"virtualmachinetemplates/status": nil,
			}
			parents := getParentResourceNames(resourcesStorage)
			Expect(parents).To(ConsistOf(virtualmachines, virtualmachinetemplates))
		})

		It("should deduplicate parents with multiple subresources", func() {
			resourcesStorage := map[string]rest.Storage{
				virtualmachines:           nil,
				"virtualmachines/process": nil,
				"virtualmachines/create":  nil,
				"virtualmachines/status":  nil,
			}
			parents := getParentResourceNames(resourcesStorage)
			Expect(parents).To(ConsistOf(virtualmachines))
		})

		It("should handle deeply nested paths by taking first segment", func() {
			resourcesStorage := map[string]rest.Storage{
				"resources/sub/nested": nil,
			}
			parents := getParentResourceNames(resourcesStorage)
			Expect(parents).To(ConsistOf("resources"))
		})
	})

	Context("filteringAPIResourceLister", func() {
		var (
			apiResourceList metav1.APIResourceList
			originalHandler restful.RouteFunction
		)

		BeforeEach(func() {
			apiResourceList = metav1.APIResourceList{
				GroupVersion: v1alpha1,
				APIResources: []metav1.APIResource{
					{Name: virtualmachines, Namespaced: true},
					{Name: "virtualmachines/process", Namespaced: true},
					{Name: "virtualmachines/create", Namespaced: true},
					{Name: virtualmachinetemplates, Namespaced: true},
					{Name: "virtualmachinetemplates/status", Namespaced: true},
				},
			}

			originalHandler = func(_ *restful.Request, resp *restful.Response) {
				resp.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(resp).Encode(apiResourceList)
			}
		})

		It("should filter out specified resources", func() {
			lister := &filteringAPIResourceLister{
				groupVersion:    v1alpha1,
				originalHandler: originalHandler,
				resourcesToHide: []string{virtualmachines},
			}

			resources := lister.ListAPIResources()
			Expect(resources).To(HaveLen(4))
			for _, r := range resources {
				Expect(r.Name).ToNot(Equal(virtualmachines))
			}
		})

		It("should filter out multiple resources", func() {
			lister := &filteringAPIResourceLister{
				groupVersion:    "v1alpha1",
				originalHandler: originalHandler,
				resourcesToHide: []string{virtualmachines, virtualmachinetemplates},
			}

			resources := lister.ListAPIResources()
			Expect(resources).To(HaveLen(3))
			for _, r := range resources {
				Expect(r.Name).ToNot(Equal(virtualmachines))
				Expect(r.Name).ToNot(Equal(virtualmachinetemplates))
			}
		})

		It("should return all resources when nothing to hide", func() {
			lister := &filteringAPIResourceLister{
				groupVersion:    v1alpha1,
				originalHandler: originalHandler,
				resourcesToHide: []string{},
			}

			resources := lister.ListAPIResources()
			Expect(resources).To(Equal(apiResourceList.APIResources))
		})

		It("should cache filtered results", func() {
			callCount := 0
			countingHandler := func(_ *restful.Request, resp *restful.Response) {
				callCount++
				resp.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(resp).Encode(apiResourceList)
			}

			lister := &filteringAPIResourceLister{
				groupVersion:    v1alpha1,
				originalHandler: countingHandler,
				resourcesToHide: []string{virtualmachines},
			}

			for range 3 {
				_ = lister.ListAPIResources()
			}

			Expect(callCount).To(Equal(1))
			Expect(lister.cached).To(BeTrue())
		})

		It("should return cached list on subsequent calls", func() {
			lister := &filteringAPIResourceLister{
				groupVersion:    "v1alpha1",
				originalHandler: originalHandler,
				resourcesToHide: []string{virtualmachines},
			}

			resources1 := lister.ListAPIResources()

			apiResourceList.APIResources = append(apiResourceList.APIResources,
				metav1.APIResource{
					Name: virtualmachinetemplaterequests,
				},
			)
			resources2 := lister.ListAPIResources()

			Expect(resources2).To(Equal(resources1))
			for _, r := range resources2 {
				Expect(r.Name).ToNot(Equal(virtualmachinetemplaterequests))
			}
		})

		It("should return empty list on handler error", func() {
			errorHandler := func(_ *restful.Request, resp *restful.Response) {
				resp.WriteHeader(http.StatusInternalServerError)
				_, _ = resp.Write([]byte("invalid json"))
			}

			lister := &filteringAPIResourceLister{
				groupVersion:    v1alpha1,
				originalHandler: errorHandler,
				resourcesToHide: []string{virtualmachines},
			}

			resources := lister.ListAPIResources()
			Expect(resources).To(BeEmpty())
			Expect(lister.cached).To(BeTrue())
		})

		It("should handle resources not in hide list", func() {
			lister := &filteringAPIResourceLister{
				groupVersion:    v1alpha1,
				originalHandler: originalHandler,
				resourcesToHide: []string{virtualmachinetemplaterequests},
			}

			resources := lister.ListAPIResources()
			Expect(resources).To(HaveLen(5))
		})
	})

	Describe("filter method", func() {
		It("should filter resources from unfiltered list", func() {
			lister := &filteringAPIResourceLister{
				resourcesToHide: []string{virtualmachines, virtualmachinetemplaterequests},
			}

			unfiltered := []metav1.APIResource{
				{Name: virtualmachines},
				{Name: virtualmachinetemplates},
				{Name: virtualmachinetemplaterequests},
			}

			filtered := lister.filter(unfiltered)
			Expect(filtered).To(HaveLen(1))
			Expect(filtered[0].Name).To(Equal(virtualmachinetemplates))
			Expect(lister.cached).To(BeTrue())
		})

		It("should handle empty unfiltered list", func() {
			lister := &filteringAPIResourceLister{
				resourcesToHide: []string{virtualmachines},
			}

			filtered := lister.filter([]metav1.APIResource{})
			Expect(filtered).To(BeEmpty())
			Expect(lister.cached).To(BeTrue())
		})

		It("should handle nil unfiltered list", func() {
			lister := &filteringAPIResourceLister{
				resourcesToHide: []string{virtualmachines},
			}

			filtered := lister.filter(nil)
			Expect(filtered).To(BeEmpty())
			Expect(lister.cached).To(BeTrue())
		})
	})

	Context("installFilteredAPIVersionHandler", func() {
		const (
			testGroup   = "subresources.template.kubevirt.io"
			testVersion = "v1beta1"
		)

		var (
			gv            schema.GroupVersion
			container     *restful.Container
			factory       runtime.NegotiatedSerializer
			fakeDiscovery discoveryendpoint.FakeResourceManager
			subresources  []metav1.APIResource
		)

		BeforeEach(func() {
			gv = schema.GroupVersion{Group: testGroup, Version: testVersion}
			container = restful.NewContainer()

			subresources = []metav1.APIResource{
				{Name: virtualmachinetemplatesProcess, Namespaced: true, Kind: processedVirtualMachineTemplate, Verbs: metav1.Verbs{create}},
				{Name: virtualmachinetemplatesCreate, Namespaced: true, Kind: processedVirtualMachineTemplate, Verbs: metav1.Verbs{create}},
			}
			allResources := append([]metav1.APIResource{
				{Name: virtualmachinetemplates, Namespaced: true, Kind: "VirtualMachineTemplate", Verbs: metav1.Verbs{"get", "list"}},
			}, subresources...)

			wsPath := path.Join(genericapiserver.APIGroupPrefix, gv.Group, gv.Version)
			ws := new(restful.WebService).Path(wsPath)
			ws.Route(ws.GET("/").To(func(_ *restful.Request, resp *restful.Response) {
				resp.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(resp).Encode(metav1.APIResourceList{GroupVersion: gv.String(), APIResources: allResources})
			}))
			container.Add(ws)

			scheme := runtime.NewScheme()
			metav1.AddToGroupVersion(scheme, schema.GroupVersion{Version: v1})
			factory = serializer.NewCodecFactory(scheme)
			fakeDiscovery = discoveryendpoint.NewFakeResourceManager()
		})

		It("should filter parent from legacy and update aggregated discovery", func() {
			expectedDiscoveryResources, err := endpoints.ConvertGroupVersionIntoToDiscovery(subresources)
			Expect(err).ToNot(HaveOccurred())
			fakeDiscovery.Expect().AddGroupVersion(gv.Group, apidiscoveryv2.APIVersionDiscovery{
				Freshness: apidiscoveryv2.DiscoveryFreshnessCurrent,
				Version:   gv.Version,
				Resources: expectedDiscoveryResources,
			})

			Expect(installFilteredAPIVersionHandler(gv, []string{virtualmachinetemplates}, container, factory, fakeDiscovery)).To(Succeed())

			wsPath := path.Join(genericapiserver.APIGroupPrefix, gv.Group, gv.Version) + "/"
			req := httptest.NewRequest(http.MethodGet, wsPath, http.NoBody)
			req.Header.Set("Accept", "application/json")
			recorder := httptest.NewRecorder()
			container.ServeHTTP(recorder, req)

			var result metav1.APIResourceList
			Expect(json.NewDecoder(recorder.Body).Decode(&result)).To(Succeed())
			for _, r := range result.APIResources {
				Expect(r.Name).ToNot(Equal(virtualmachinetemplates))
			}

			Expect(fakeDiscovery.Validate()).To(Succeed())
		})

		It("should return error when WebService not found", func() {
			wrongGV := schema.GroupVersion{Group: "nonexistent.io", Version: v1}
			err := installFilteredAPIVersionHandler(wrongGV, []string{virtualmachinetemplates}, container, factory, fakeDiscovery)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not find the APIResource WebService"))
		})

		It("should return error when route not found", func() {
			emptyWs := new(restful.WebService).Path(path.Join(genericapiserver.APIGroupPrefix, gv.Group, gv.Version))
			emptyContainer := restful.NewContainer()
			emptyContainer.Add(emptyWs)

			err := installFilteredAPIVersionHandler(gv, []string{virtualmachinetemplates}, emptyContainer, factory, fakeDiscovery)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not find the APIVersionHandler"))
		})
	})

	Describe("get method", func() {
		It("should parse APIResourceList from handler response", func() {
			expectedResources := []metav1.APIResource{
				{Name: virtualmachines, Namespaced: true},
				{Name: virtualmachinetemplates, Namespaced: true},
			}

			handler := func(_ *restful.Request, resp *restful.Response) {
				resp.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(resp).Encode(metav1.APIResourceList{
					GroupVersion: v1,
					APIResources: expectedResources,
				})
			}

			lister := &filteringAPIResourceLister{
				originalHandler: handler,
			}

			resources, err := lister.get()
			Expect(err).ToNot(HaveOccurred())
			Expect(resources).To(Equal(expectedResources))
		})

		It("should return error for invalid JSON", func() {
			handler := func(_ *restful.Request, resp *restful.Response) {
				recorder := resp.ResponseWriter.(*httptest.ResponseRecorder)
				recorder.Body.WriteString("invalid json{")
			}

			lister := &filteringAPIResourceLister{
				originalHandler: handler,
			}

			resources, err := lister.get()
			Expect(err).To(HaveOccurred())
			Expect(resources).To(BeNil())
		})
	})
})
