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

package tests_test

import (
	"context"
	"net/http"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"

	"k8s.io/client-go/rest"

	templateapi "kubevirt.io/virt-template-api/core"
)

const securityHeadersRequestTimeout = 30 * time.Second

var _ = Describe("Security headers", func() {
	var httpClient *http.Client

	BeforeEach(func() {
		transport, err := rest.TransportFor(virtClient.Config())
		Expect(err).ToNot(HaveOccurred())
		httpClient = &http.Client{Transport: transport}
	})

	DescribeTable(
		"should be present on the endpoints scanned by RapidAST",
		func(path string, expectedStatus types.GomegaMatcher) {
			ctx, cancel := context.WithTimeout(context.Background(), securityHeadersRequestTimeout)
			defer cancel()

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, virtClient.Config().Host+path, http.NoBody)
			Expect(err).ToNot(HaveOccurred())

			resp, err := httpClient.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(expectedStatus)

			xcto := resp.Header.Values("X-Content-Type-Options")
			Expect(strings.ToLower(strings.Join(xcto, ", "))).To(
				ContainSubstring("nosniff"),
				"X-Content-Type-Options %v should contain nosniff", xcto,
			)

			cc := resp.Header.Values("Cache-Control")
			joined := strings.ToLower(strings.Join(cc, ", "))
			for _, directive := range []string{"no-cache", "no-store", "must-revalidate"} {
				Expect(joined).To(
					ContainSubstring(directive),
					"Cache-Control %v should contain %q", cc, directive,
				)
			}
		},
		Entry("v1beta1 discovery", "/apis/subresources.template.kubevirt.io/v1beta1/", Equal(http.StatusOK)),
		Entry("v1alpha1 discovery", "/apis/subresources.template.kubevirt.io/v1alpha1/", Equal(http.StatusOK)),
		Entry(
			"unknown path under the group returns a non-2xx response",
			"/apis/subresources.template.kubevirt.io/v1beta1/nonexistent",
			Equal(http.StatusNotFound),
		),
		Entry(
			"a real subresource endpoint hit with the wrong HTTP method",
			"/apis/subresources.template.kubevirt.io/v1beta1/namespaces/"+NamespaceTest+"/"+
				templateapi.PluralResourceName+"/does-not-exist/process",
			BeNumerically(">=", http.StatusBadRequest),
		),
	)
})
