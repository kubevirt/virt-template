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

package controller_test

import (
	"context"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"kubevirt.io/virt-template-api/core/v1beta1"

	"kubevirt.io/virt-template/internal/controller"
)

var _ = Describe("VMTRAvailabilityController", func() {
	var (
		env *envtest.Environment
		ac  *controller.VMTRAvailabilityController
	)

	startController := func() (chan error, context.CancelFunc) {
		ctrlCtx, ctrlCancel := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() {
			done <- ac.Start(ctrlCtx)
		}()
		return done, ctrlCancel
	}

	BeforeEach(func() {
		env = startNoCRDEnv()
		ac = &controller.VMTRAvailabilityController{
			Manager:         startManager(env.Config),
			DiscoveryClient: discovery.NewDiscoveryClientForConfigOrDie(env.Config),
		}
		ac.SetPollInterval(1 * time.Second)
	})

	It("should start VMTR controller once required CRDs become available", func() {
		done, ctrlCancel := startController()
		DeferCleanup(ctrlCancel)
		Consistently(done, 2*time.Second).ShouldNot(Receive())

		_, err := envtest.InstallCRDs(env.Config, envtest.CRDInstallOptions{
			Paths: []string{
				filepath.Join("..", "..", "config", "crd", "testing"),
			},
			Scheme: testScheme,
		})
		Expect(err).ToNot(HaveOccurred())

		Eventually(done, 60*time.Second).Should(Receive(BeNil()))
	})

	It("should set status on pending VirtualMachineTemplateRequests while waiting", func() {
		cl, err := client.New(env.Config, client.Options{Scheme: testScheme})
		Expect(err).NotTo(HaveOccurred())

		Expect(cl.Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: testNamespace},
		})).To(Succeed())
		Expect(cl.Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: testVMNamespace},
		})).To(Succeed())

		tplReq := createRequest(cl, testNamespace, testVMNamespace)

		done, ctrlCancel := startController()
		DeferCleanup(ctrlCancel)

		Eventually(func(g Gomega) {
			g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
			gExpectCondition(g, tplReq, v1beta1.ConditionReady, metav1.ConditionFalse, v1beta1.ReasonWaiting,
				ContainSubstring("Required CRDs are not yet available"))
			gExpectCondition(g, tplReq, v1beta1.ConditionProgressing, metav1.ConditionTrue, v1beta1.ReasonWaiting)
		}, 60*time.Second).Should(Succeed())

		ctrlCancel()
		Eventually(done, 60*time.Second).Should(Receive(BeNil()))
	})

	It("should stop polling when context is canceled", func() {
		done, ctrlCancel := startController()
		Consistently(done, 2*time.Second).ShouldNot(Receive())
		ctrlCancel()
		Eventually(done, 60*time.Second).Should(Receive(BeNil()))
	})
})

func startNoCRDEnv() *envtest.Environment {
	env := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crd", "bases"),
		},
		ErrorIfCRDPathMissing: true,
		Scheme:                testScheme,
	}
	if dir := getFirstFoundEnvTestBinaryDir(); dir != "" {
		env.BinaryAssetsDirectory = dir
	}

	_, err := env.Start()
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(func() {
		Expect(env.Stop()).To(Succeed())
	})

	return env
}

func startManager(cfg *rest.Config) ctrl.Manager {
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: testScheme,
	})
	Expect(err).NotTo(HaveOccurred())

	mgrCtx, mgrCancel := context.WithCancel(ctx) //nolint:gosec
	DeferCleanup(mgrCancel)
	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(mgrCtx)).To(Succeed())
	}()

	return mgr
}
