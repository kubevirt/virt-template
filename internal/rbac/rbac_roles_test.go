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

package rbac_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/yaml"

	"kubevirt.io/virt-template-api/core/v1alpha1"
	templateclient "kubevirt.io/virt-template-client-go/virttemplate"
)

func TestRBACRoles(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RBAC Roles Test Suite")
}

var (
	testEnv       *envtest.Environment
	cfg           *rest.Config
	k8sClient     kubernetes.Interface
	clusterRoles  map[string]*rbacv1.ClusterRole
	testNamespace string
)

var _ = BeforeSuite(func() {
	testScheme := runtime.NewScheme()
	Expect(corev1.AddToScheme(testScheme)).To(Succeed())
	Expect(rbacv1.AddToScheme(testScheme)).To(Succeed())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crd", "bases"),
		},
		ErrorIfCRDPathMissing: false,
		Scheme:                testScheme,
	}

	if getFirstFoundEnvTestBinaryDir() != "" {
		testEnv.BinaryAssetsDirectory = getFirstFoundEnvTestBinaryDir()
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	k8sClient, err = kubernetes.NewForConfig(cfg)
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	clusterRoles = make(map[string]*rbacv1.ClusterRole)
	rbacDir := filepath.Join("..", "..", "config", "rbac")
	roleFiles, err := filepath.Glob(filepath.Join(rbacDir, "*_role.yaml"))
	Expect(err).NotTo(HaveOccurred())
	Expect(roleFiles).NotTo(BeEmpty(), "No role files found matching *_role.yaml pattern")

	for _, path := range roleFiles {
		data, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())

		var role rbacv1.ClusterRole
		Expect(yaml.Unmarshal(data, &role)).To(Succeed())

		_, err = k8sClient.RbacV1().ClusterRoles().Create(context.Background(), &role, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		clusterRoles[role.Name] = &role
	}
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	Expect(testEnv.Stop()).To(Succeed())
})

var _ = Describe("RBAC Roles", func() {
	var clusterRoleBindings []*rbacv1.ClusterRoleBinding

	BeforeEach(func() {
		clusterRoleBindings = []*rbacv1.ClusterRoleBinding{}
		testNamespace = "test-ns-" + rand.String(5)
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace,
			},
		}
		_, err := k8sClient.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		for _, crb := range clusterRoleBindings {
			err := k8sClient.RbacV1().ClusterRoleBindings().Delete(context.Background(), crb.Name, metav1.DeleteOptions{})
			if err != nil && !k8serrors.IsNotFound(err) {
				Expect(err).NotTo(HaveOccurred())
			}
		}
		err := k8sClient.CoreV1().Namespaces().Delete(context.Background(), testNamespace, metav1.DeleteOptions{})
		if err != nil && !k8serrors.IsNotFound(err) {
			Expect(err).NotTo(HaveOccurred())
		}
	})

	createServiceAccount := func(name string) *corev1.ServiceAccount {
		sa := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: testNamespace,
			},
		}
		sa, err := k8sClient.CoreV1().ServiceAccounts(testNamespace).Create(context.Background(), sa, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		return sa
	}

	createClusterRoleBinding := func(name, roleName string, sa *corev1.ServiceAccount) *rbacv1.ClusterRoleBinding {
		crb := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     roleName,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      sa.Name,
					Namespace: sa.Namespace,
				},
			},
		}
		crb, err := k8sClient.RbacV1().ClusterRoleBindings().Create(context.Background(), crb, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		clusterRoleBindings = append(clusterRoleBindings, crb)
		return crb
	}

	createClientForServiceAccount := func(sa *corev1.ServiceAccount) templateclient.Interface {
		cfgCopy := rest.CopyConfig(cfg)
		cfgCopy.Impersonate = rest.ImpersonationConfig{
			UserName: "system:serviceaccount:" + sa.Namespace + ":" + sa.Name,
		}
		client, err := templateclient.NewForConfig(cfgCopy)
		Expect(err).NotTo(HaveOccurred())
		return client
	}

	createVirtualMachineTemplate := func(client templateclient.Interface, name string) error {
		tpl := &v1alpha1.VirtualMachineTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: testNamespace,
			},
			Spec: v1alpha1.VirtualMachineTemplateSpec{
				VirtualMachine: &runtime.RawExtension{
					Object: &unstructured.Unstructured{
						Object: map[string]any{
							"apiVersion": "kubevirt.io/v1",
							"kind":       "VirtualMachine",
						},
					},
				},
			},
		}
		_, err := client.TemplateV1alpha1().VirtualMachineTemplates(testNamespace).
			Create(context.Background(), tpl, metav1.CreateOptions{})
		return err
	}

	createVirtualMachineTemplateRequest := func(client templateclient.Interface, name string) error {
		tplReq := &v1alpha1.VirtualMachineTemplateRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: testNamespace,
			},
			Spec: v1alpha1.VirtualMachineTemplateRequestSpec{
				VirtualMachineRef: v1alpha1.VirtualMachineReference{
					Namespace: testNamespace,
					Name:      "test-vm",
				},
			},
		}
		_, err := client.TemplateV1alpha1().VirtualMachineTemplateRequests(testNamespace).
			Create(context.Background(), tplReq, metav1.CreateOptions{})
		return err
	}

	Context("VirtualMachineTemplate roles", func() {
		DescribeTable("should allow creating, updating, and deleting VirtualMachineTemplate",
			func(roleName string, rolePrefix string) {
				role := clusterRoles[roleName]
				Expect(role).NotTo(BeNil())

				createClientForRole := func(suffix string) templateclient.Interface {
					sa := createServiceAccount(rolePrefix + suffix)
					createClusterRoleBinding(rolePrefix+suffix, role.Name, sa)
					return createClientForServiceAccount(sa)
				}

				By("should allow creating", func() {
					client := createClientForRole("-sa")
					Expect(createVirtualMachineTemplate(client, "test-template-"+rolePrefix)).To(Succeed())
				})

				By("should allow updating", func() {
					client := createClientForRole("-sa-update")
					templateName := "test-template-" + rolePrefix + "-update"
					Expect(createVirtualMachineTemplate(client, templateName)).To(Succeed())

					tpl, err := client.TemplateV1alpha1().VirtualMachineTemplates(testNamespace).
						Get(context.Background(), templateName, metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					tpl.Labels = map[string]string{"test": "label"}
					_, err = client.TemplateV1alpha1().VirtualMachineTemplates(testNamespace).
						Update(context.Background(), tpl, metav1.UpdateOptions{})
					Expect(err).NotTo(HaveOccurred())
				})

				By("should allow deleting", func() {
					client := createClientForRole("-sa-delete")
					templateName := "test-template-" + rolePrefix + "-delete"
					Expect(createVirtualMachineTemplate(client, templateName)).To(Succeed())

					err := client.TemplateV1alpha1().VirtualMachineTemplates(testNamespace).
						Delete(context.Background(), templateName, metav1.DeleteOptions{})
					Expect(err).NotTo(HaveOccurred())
				})
			},
			Entry("Admin role", "virtualmachinetemplate-admin-role", "admin"),
			Entry("Editor role", "virtualmachinetemplate-editor-role", "editor"),
		)

		DescribeTable("Viewer role permissions",
			func(roleName string, suffix string, testFunc func(templateclient.Interface)) {
				role := clusterRoles[roleName]
				Expect(role).NotTo(BeNil())
				viewerSA := createServiceAccount("viewer-sa" + suffix)
				createClusterRoleBinding("viewer-crb"+suffix, role.Name, viewerSA)
				viewerClient := createClientForServiceAccount(viewerSA)
				testFunc(viewerClient)
			},
			Entry("should allow getting VirtualMachineTemplate", "virtualmachinetemplate-viewer-role", "", func(viewerClient templateclient.Interface) {
				adminRole := clusterRoles["virtualmachinetemplate-admin-role"]
				adminSA := createServiceAccount("admin-sa-viewer-setup")
				createClusterRoleBinding("admin-crb-viewer-setup", adminRole.Name, adminSA)

				adminClient := createClientForServiceAccount(adminSA)
				Expect(createVirtualMachineTemplate(adminClient, "test-template-viewer")).To(Succeed())

				_, err := viewerClient.TemplateV1alpha1().VirtualMachineTemplates(testNamespace).
					Get(context.Background(), "test-template-viewer", metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
			}),
			Entry("should allow listing VirtualMachineTemplates", "virtualmachinetemplate-viewer-role", "-list", func(viewerClient templateclient.Interface) {
				_, err := viewerClient.TemplateV1alpha1().VirtualMachineTemplates(testNamespace).
					List(context.Background(), metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
			}),
			Entry("should deny creating VirtualMachineTemplate", "virtualmachinetemplate-viewer-role", "-create", func(viewerClient templateclient.Interface) {
				err := createVirtualMachineTemplate(viewerClient, "test-template-viewer-create")
				Expect(err).To(HaveOccurred())
				Expect(k8serrors.IsForbidden(err)).To(BeTrue())
			}),
		)
	})

	Context("VirtualMachineTemplateRequest roles", func() {
		DescribeTable("should allow creating VirtualMachineTemplateRequest",
			func(roleName string, rolePrefix string) {
				role := clusterRoles[roleName]
				Expect(role).NotTo(BeNil())
				sa := createServiceAccount("vmtr-" + rolePrefix + "-sa")
				createClusterRoleBinding("vmtr-"+rolePrefix+"-crb", role.Name, sa)

				client := createClientForServiceAccount(sa)
				err := createVirtualMachineTemplateRequest(client, "test-vmtr-"+rolePrefix)
				if err != nil {
					Expect(k8serrors.IsForbidden(err)).To(BeFalse(), "Should not be forbidden by RBAC")
				}
			},
			Entry("Admin role", "virtualmachinetemplaterequest-admin-role", "admin"),
			Entry("Editor role", "virtualmachinetemplaterequest-editor-role", "editor"),
		)

		DescribeTable("Viewer role permissions",
			func(roleName string, testFunc func(templateclient.Interface)) {
				role := clusterRoles[roleName]
				Expect(role).NotTo(BeNil())
				viewerSA := createServiceAccount("vmtr-viewer-sa")
				createClusterRoleBinding("vmtr-viewer-crb", role.Name, viewerSA)
				viewerClient := createClientForServiceAccount(viewerSA)
				testFunc(viewerClient)
			},
			Entry("should deny creating VirtualMachineTemplateRequest", "virtualmachinetemplaterequest-viewer-role", func(viewerClient templateclient.Interface) {
				err := createVirtualMachineTemplateRequest(viewerClient, "test-vmtr-viewer")
				Expect(err).To(HaveOccurred())
				Expect(k8serrors.IsForbidden(err)).To(BeTrue())
			}),
		)
	})
})

func getFirstFoundEnvTestBinaryDir() string {
	basePath := filepath.Join("..", "..", "bin", "k8s")
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return filepath.Join(basePath, entry.Name())
		}
	}
	return ""
}
