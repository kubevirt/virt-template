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
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	ginkgo_reporters "github.com/onsi/ginkgo/v2/reporters"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"

	"kubevirt.io/client-go/kubecli"
	qe_reporters "kubevirt.io/qe-tools/pkg/ginkgo-reporters"

	templateclient "kubevirt.io/virt-template-client-go/virttemplate"
)

var (
	// testNamespace and testSecondaryNamespace are created for every spec and deleted after it,
	// with everything in them.
	testNamespace          string
	testSecondaryNamespace string

	virtClient kubecli.KubevirtClient
	tplClient  templateclient.Interface

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

	tplClient, err = templateclient.NewForConfig(virtClient.Config())
	Expect(err).NotTo(HaveOccurred())
})

var _ = BeforeEach(func() {
	testNamespace = createNamespace()
	testSecondaryNamespace = createNamespace()
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

// createNamespace creates a namespace that is deleted, with everything in it, after the spec.
func createNamespace() string {
	GinkgoHelper()
	namespace, err := virtClient.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{GenerateName: "vm-template-test-"},
	}, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(func() {
		Expect(virtClient.CoreV1().Namespaces().Delete(context.Background(), namespace.Name, metav1.DeleteOptions{})).
			To(Or(Succeed(), MatchError(k8serrors.IsNotFound, "k8serrors.IsNotFound")))
	})
	return namespace.Name
}
