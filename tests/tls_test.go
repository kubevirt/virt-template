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
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Only the NIST curves are checked, since they are FIPS-approved and thus
// guaranteed to be available regardless of whether the cluster runs Go's
// FIPS-restricted crypto.
var _ = Describe("TLS", func() {
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

	// Serial: these entries mutate the apiserver/controller-manager
	// deployments in place, so they must not run concurrently with any other
	// tests that talk to those pods.
	DescribeTable(
		"restricting TLS curves via --tls-curve-preferences",
		Serial,
		func(controlPlane string, remotePort int, allowed, excluded []tls.CurveID) {
			setTLSCurvePreferences(controlPlane, allowed...)

			localPort := forwardPodPort(controlPlane, remotePort)

			for _, curveID := range allowed {
				conn, err := tls.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", localPort), &tls.Config{
					InsecureSkipVerify: true, //nolint:gosec // handshake tests curve negotiation, not certificate validity
					MinVersion:         tls.VersionTLS13,
					CurvePreferences:   []tls.CurveID{curveID},
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(conn.ConnectionState().CurveID).To(Equal(curveID))
				Expect(conn.Close()).To(Succeed())
			}

			for _, curveID := range excluded {
				_, err := tls.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", localPort), &tls.Config{
					InsecureSkipVerify: true, //nolint:gosec // handshake tests curve negotiation, not certificate validity
					MinVersion:         tls.VersionTLS13,
					CurvePreferences:   []tls.CurveID{curveID},
				})
				Expect(err).To(MatchError(ContainSubstring("handshake failure")))
			}
		},
		Entry("API server should only accept CurveP256", "apiserver", 9443,
			[]tls.CurveID{tls.CurveP256}, []tls.CurveID{tls.CurveP384, tls.CurveP521}),
		Entry("API server should only accept CurveP384", "apiserver", 9443,
			[]tls.CurveID{tls.CurveP384}, []tls.CurveID{tls.CurveP256, tls.CurveP521}),
		Entry("API server should accept CurveP256 and CurveP521 but not CurveP384", "apiserver", 9443,
			[]tls.CurveID{tls.CurveP256, tls.CurveP521}, []tls.CurveID{tls.CurveP384}),
		Entry("controller manager should only accept CurveP256", "controller-manager", 8443,
			[]tls.CurveID{tls.CurveP256}, []tls.CurveID{tls.CurveP384, tls.CurveP521}),
		Entry("controller manager should only accept CurveP384", "controller-manager", 8443,
			[]tls.CurveID{tls.CurveP384}, []tls.CurveID{tls.CurveP256, tls.CurveP521}),
		Entry("controller manager should accept CurveP256 and CurveP521 but not CurveP384", "controller-manager", 8443,
			[]tls.CurveID{tls.CurveP256, tls.CurveP521}, []tls.CurveID{tls.CurveP384}),
	)
})

// setTLSCurvePreferences configures custom TLS curves for the deployment and
// restores its original arguments when the spec finishes.
func setTLSCurvePreferences(controlPlane string, curveIDs ...tls.CurveID) {
	GinkgoHelper()

	deployments, err := virtClient.AppsV1().Deployments(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app.kubernetes.io/name=virt-template,control-plane=%s", controlPlane),
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(deployments.Items).To(HaveLen(1), fmt.Sprintf("expected exactly one %s deployment", controlPlane))
	deployment := &deployments.Items[0]
	originalArgs := slices.Clone(deployment.Spec.Template.Spec.Containers[0].Args)
	DeferCleanup(patchDeploymentArgsAndWait, deployment, originalArgs)

	args := make([]string, 0, len(originalArgs)+1)
	for i := 0; i < len(originalArgs); i++ {
		arg := originalArgs[i]
		if arg == "--tls-curve-preferences" {
			i++
			continue
		}
		if !strings.HasPrefix(arg, "--tls-curve-preferences=") {
			args = append(args, arg)
		}
	}

	ids := make([]string, len(curveIDs))
	for i, curveID := range curveIDs {
		ids[i] = strconv.Itoa(int(curveID))
	}
	args = append(args, "--tls-curve-preferences="+strings.Join(ids, ","))

	patchDeploymentArgsAndWait(deployment, args)
}

func patchDeploymentArgsAndWait(deployment *appsv1.Deployment, args []string) {
	GinkgoHelper()

	patch := client.StrategicMergeFrom(deployment.DeepCopy())
	deployment.Spec.Template.Spec.Containers[0].Args = args
	patchBytes, err := patch.Data(deployment)
	Expect(err).ToNot(HaveOccurred())

	deployment, err = virtClient.AppsV1().Deployments(deployment.Namespace).Patch(
		context.Background(), deployment.Name, patch.Type(), patchBytes, metav1.PatchOptions{},
	)
	Expect(err).ToNot(HaveOccurred())

	replicas := int32(1)
	if deployment.Spec.Replicas != nil {
		replicas = *deployment.Spec.Replicas
	}

	Eventually(func(g Gomega) {
		current, getErr := virtClient.AppsV1().Deployments(deployment.Namespace).Get(
			context.Background(), deployment.Name, metav1.GetOptions{},
		)
		g.Expect(getErr).ToNot(HaveOccurred())
		g.Expect(current.Status.ObservedGeneration).To(BeNumerically(">=", deployment.Generation))
		g.Expect(current.Status.UpdatedReplicas).To(Equal(replicas))
		g.Expect(current.Status.Replicas).To(Equal(replicas))
		g.Expect(current.Status.AvailableReplicas).To(Equal(replicas))
		// The new pod needs to pass its startup and readiness probes, and the
		// old pod needs to terminate, before the rollout is considered done.
	}, 90*time.Second, 2*time.Second).Should(Succeed())
}

// forwardPodPort port-forwards to the given port of a running pod matching
// the control-plane label and returns the chosen local port.
func forwardPodPort(controlPlane string, remotePort int) (localPort uint16) {
	GinkgoHelper()

	var pod *corev1.Pod
	Eventually(func(g Gomega) *corev1.Pod {
		pods, err := virtClient.CoreV1().Pods(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{
			LabelSelector: fmt.Sprintf("app.kubernetes.io/name=virt-template,control-plane=%s", controlPlane),
		})
		g.Expect(err).ToNot(HaveOccurred())

		pod = nil
		for i := range pods.Items {
			// A Terminating pod (e.g. the old pod during a rollout) can still
			// report Ready: True until the kubelet actually tears it down, so
			// it must be excluded explicitly.
			if pods.Items[i].DeletionTimestamp != nil {
				continue
			}
			for _, cond := range pods.Items[i].Status.Conditions {
				if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
					pod = &pods.Items[i]
					break
				}
			}
			if pod != nil {
				break
			}
		}
		return pod
	}, 30*time.Second, time.Second).ShouldNot(BeNil(), fmt.Sprintf("no ready %s pod found", controlPlane))

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
	localPort = ports[0].Local

	var tcpDialer net.Dialer
	Eventually(func() error {
		conn, dialErr := tcpDialer.DialContext(context.Background(), "tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
		if dialErr != nil {
			return dialErr
		}
		return conn.Close()
	}, 30*time.Second, time.Second).Should(Succeed())

	return localPort
}
