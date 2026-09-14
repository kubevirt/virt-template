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
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// The allowed key exchange curves can be restricted via the
// --tls-curve-preferences flag on the controller manager (see cmd/main.go),
// and the API server gets the equivalent flag for free from
// k8s.io/apiserver (see internal/apiserver). Neither is set by the default
// deployment, so both servers support Go's default curves. These tests
// restrict the client to a single curve at a time and verify the server
// negotiates that exact curve, proving each curve is actually supported
// and correctly selected rather than just that a handshake completes.
// Only the NIST curves are checked, since they are FIPS-approved and thus
// guaranteed to be available regardless of whether the cluster runs Go's
// FIPS-restricted crypto.
var _ = Describe("TLS", Ordered, func() {
	DescribeTable(
		"negotiating TLS curves",
		func(controlPlane string, remotePort int, curveID tls.CurveID) {
			localPort := forwardPodPort(controlPlane, remotePort)

			conn, err := tls.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", localPort), &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec // handshake tests curve negotiation, not certificate validity
				MinVersion:         tls.VersionTLS13,
				CurvePreferences:   []tls.CurveID{curveID},
			})
			Expect(err).ToNot(HaveOccurred())
			defer conn.Close()

			Expect(conn.ConnectionState().CurveID).To(Equal(curveID))
		},
		Entry("API server should negotiate CurveP256", "apiserver", 9443, tls.CurveP256),
		Entry("API server should negotiate CurveP384", "apiserver", 9443, tls.CurveP384),
		Entry("API server should negotiate CurveP521", "apiserver", 9443, tls.CurveP521),
		Entry("controller manager should negotiate CurveP256", "controller-manager", 8443, tls.CurveP256),
		Entry("controller manager should negotiate CurveP384", "controller-manager", 8443, tls.CurveP384),
		Entry("controller manager should negotiate CurveP521", "controller-manager", 8443, tls.CurveP521),
	)
})

// forwardPodPort port-forwards to the given port of a running pod matching
// the control-plane label and returns the chosen local port.
func forwardPodPort(controlPlane string, remotePort int) (localPort uint16) {
	GinkgoHelper()

	var pod *corev1.Pod
	Eventually(func() *corev1.Pod {
		pods, err := virtClient.CoreV1().Pods(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{
			LabelSelector: fmt.Sprintf("app.kubernetes.io/name=virt-template,control-plane=%s", controlPlane),
		})
		Expect(err).ToNot(HaveOccurred())

		pod = nil
		for i := range pods.Items {
			if pods.Items[i].Status.Phase == corev1.PodRunning {
				pod = &pods.Items[i]
				break
			}
		}
		return pod
	}, 30*time.Second, time.Second).ShouldNot(BeNil(), fmt.Sprintf("no running %s pod found", controlPlane))

	roundTripper, upgrader, err := spdy.RoundTripperFor(virtClient.Config())
	Expect(err).ToNot(HaveOccurred())

	req := virtClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(pod.Namespace).
		Name(pod.Name).
		SubResource("portforward")
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, http.MethodPost, req.URL())

	stopCh := make(chan struct{})
	DeferCleanup(func() { close(stopCh) })
	readyCh := make(chan struct{})
	fw, err := portforward.New(dialer, []string{fmt.Sprintf("0:%d", remotePort)}, stopCh, readyCh, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		if forwardErr := fw.ForwardPorts(); forwardErr != nil {
			_, _ = fmt.Fprintf(GinkgoWriter, "port-forward stopped: %v\n", forwardErr)
		}
	}()

	Eventually(readyCh, 10*time.Second).Should(BeClosed())

	ports, err := fw.GetPorts()
	Expect(err).ToNot(HaveOccurred())
	Expect(ports).ToNot(BeEmpty())

	return ports[0].Local
}
