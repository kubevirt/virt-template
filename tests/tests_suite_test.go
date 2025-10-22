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
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	ginkgo_reporters "github.com/onsi/ginkgo/v2/reporters"

	"k8s.io/client-go/tools/clientcmd"

	"kubevirt.io/client-go/kubecli"
	qe_reporters "kubevirt.io/qe-tools/pkg/ginkgo-reporters"
)

var (
	virtClient          kubecli.KubevirtClient
	afterSuiteReporters []Reporter
)

var _ = BeforeSuite(func() {
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
		Skip("KUBECONFIG not found, skipping tests")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	Expect(err).NotTo(HaveOccurred())

	virtClient, err = kubecli.GetKubevirtClientFromRESTConfig(config)
	Expect(err).NotTo(HaveOccurred())
})

var _ = ReportAfterSuite("TestFunctional", func(report Report) {
	for _, reporter := range afterSuiteReporters {
		//nolint:staticcheck
		ginkgo_reporters.ReportViaDeprecatedReporter(reporter, report)
	}
})

func TestFunctional(t *testing.T) {
	if qe_reporters.JunitOutput != "" {
		afterSuiteReporters = append(afterSuiteReporters, ginkgo_reporters.NewJUnitReporter(qe_reporters.JunitOutput))
	}
	if qe_reporters.Polarion.Run {
		afterSuiteReporters = append(afterSuiteReporters, &qe_reporters.Polarion)
	}

	RegisterFailHandler(Fail)
	RunSpecs(t, "Functional test suite")
}
