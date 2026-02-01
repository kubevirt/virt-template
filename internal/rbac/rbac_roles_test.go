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
	roleFiles := []string{
		"virtualmachinetemplate_admin_role.yaml",
		"virtualmachinetemplate_editor_role.yaml",
		"virtualmachinetemplate_viewer_role.yaml",
		"virtualmachinetemplaterequest_admin_role.yaml",
		"virtualmachinetemplaterequest_editor_role.yaml",
		"virtualmachinetemplaterequest_viewer_role.yaml",
	}

	for _, filename := range roleFiles {
		path := filepath.Join(rbacDir, filename)
		data, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())

		var role rbacv1.ClusterRole
		Expect(yaml.Unmarshal(data, &role)).To(Succeed())

		_, err = k8sClient.RbacV1().ClusterRoles().Create(context.Background(), &role, metav1.CreateOptions{})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			Expect(err).NotTo(HaveOccurred())
		}

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

	testRoleCreateUpdateDelete := func(getRole func() *rbacv1.ClusterRole, rolePrefix string) {
		It("should allow creating", func() {
			role := getRole()
			sa := createServiceAccount(rolePrefix + "-sa")
			createClusterRoleBinding(rolePrefix+"-crb", role.Name, sa)

			client := createClientForServiceAccount(sa)
			Expect(createVirtualMachineTemplate(client, "test-template-"+rolePrefix)).To(Succeed())
		})

		It("should allow updating", func() {
			role := getRole()
			sa := createServiceAccount(rolePrefix + "-sa-update")
			createClusterRoleBinding(rolePrefix+"-crb-update", role.Name, sa)

			client := createClientForServiceAccount(sa)
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

		It("should allow deleting", func() {
			role := getRole()
			sa := createServiceAccount(rolePrefix + "-sa-delete")
			createClusterRoleBinding(rolePrefix+"-crb-delete", role.Name, sa)

			client := createClientForServiceAccount(sa)
			templateName := "test-template-" + rolePrefix + "-delete"
			Expect(createVirtualMachineTemplate(client, templateName)).To(Succeed())

			err := client.TemplateV1alpha1().VirtualMachineTemplates(testNamespace).
				Delete(context.Background(), templateName, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		})
	}

	Context("VirtualMachineTemplate roles", func() {
		Describe("Admin role", func() {
			testRoleCreateUpdateDelete(func() *rbacv1.ClusterRole { return clusterRoles["virtualmachinetemplate-admin-role"] }, "admin")
		})

		Describe("Editor role", func() {
			testRoleCreateUpdateDelete(func() *rbacv1.ClusterRole { return clusterRoles["virtualmachinetemplate-editor-role"] }, "editor")
		})

		Describe("Viewer role", func() {
			It("should allow getting VirtualMachineTemplate", func() {
				adminRole := clusterRoles["virtualmachinetemplate-admin-role"]
				viewerRole := clusterRoles["virtualmachinetemplate-viewer-role"]
				adminSA := createServiceAccount("admin-sa-viewer-setup")
				createClusterRoleBinding("admin-crb-viewer-setup", adminRole.Name, adminSA)

				adminClient := createClientForServiceAccount(adminSA)
				Expect(createVirtualMachineTemplate(adminClient, "test-template-viewer")).To(Succeed())

				viewerSA := createServiceAccount("viewer-sa")
				createClusterRoleBinding("viewer-crb", viewerRole.Name, viewerSA)

				viewerClient := createClientForServiceAccount(viewerSA)
				_, err := viewerClient.TemplateV1alpha1().VirtualMachineTemplates(testNamespace).
					Get(context.Background(), "test-template-viewer", metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
			})

			It("should allow listing VirtualMachineTemplates", func() {
				viewerRole := clusterRoles["virtualmachinetemplate-viewer-role"]
				viewerSA := createServiceAccount("viewer-sa-list")
				createClusterRoleBinding("viewer-crb-list", viewerRole.Name, viewerSA)

				viewerClient := createClientForServiceAccount(viewerSA)
				_, err := viewerClient.TemplateV1alpha1().VirtualMachineTemplates(testNamespace).
					List(context.Background(), metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
			})

			It("should deny creating VirtualMachineTemplate", func() {
				viewerRole := clusterRoles["virtualmachinetemplate-viewer-role"]
				viewerSA := createServiceAccount("viewer-sa-create")
				createClusterRoleBinding("viewer-crb-create", viewerRole.Name, viewerSA)

				viewerClient := createClientForServiceAccount(viewerSA)
				err := createVirtualMachineTemplate(viewerClient, "test-template-viewer-create")
				Expect(err).To(HaveOccurred())
				Expect(k8serrors.IsForbidden(err)).To(BeTrue())
			})
		})
	})

	Context("RBAC enforcement verification", func() {
		It("should deny access when user has no permissions", func() {
			sa := createServiceAccount("no-permissions-sa")

			client := createClientForServiceAccount(sa)
			err := createVirtualMachineTemplate(client, "test-template-no-perms")
			Expect(err).To(HaveOccurred())
			Expect(k8serrors.IsForbidden(err)).To(BeTrue(), "User without permissions should be forbidden")
		})

		It("should deny access when user has no role binding", func() {
			sa := createServiceAccount("no-binding-sa")

			client := createClientForServiceAccount(sa)
			err := createVirtualMachineTemplateRequest(client, "test-vmtr-no-binding")
			Expect(err).To(HaveOccurred())
			Expect(k8serrors.IsForbidden(err)).To(BeTrue(), "User without role binding should be forbidden")
		})
	})

	testVMTRRoleCreate := func(getRole func() *rbacv1.ClusterRole, rolePrefix string) {
		It("should allow creating VirtualMachineTemplateRequest", func() {
			role := getRole()
			sa := createServiceAccount("vmtr-" + rolePrefix + "-sa")
			createClusterRoleBinding("vmtr-"+rolePrefix+"-crb", role.Name, sa)

			client := createClientForServiceAccount(sa)
			err := createVirtualMachineTemplateRequest(client, "test-vmtr-"+rolePrefix)
			if err != nil {
				Expect(k8serrors.IsForbidden(err)).To(BeFalse(), "Should not be forbidden by RBAC")
			}
		})
	}

	Context("VirtualMachineTemplateRequest roles", func() {
		Describe("Admin role", func() {
			testVMTRRoleCreate(func() *rbacv1.ClusterRole { return clusterRoles["virtualmachinetemplaterequest-admin-role"] }, "admin")
		})

		Describe("Editor role", func() {
			testVMTRRoleCreate(func() *rbacv1.ClusterRole { return clusterRoles["virtualmachinetemplaterequest-editor-role"] }, "editor")
		})

		Describe("Viewer role", func() {
			It("should deny creating VirtualMachineTemplateRequest", func() {
				viewerRole := clusterRoles["virtualmachinetemplaterequest-viewer-role"]
				viewerSA := createServiceAccount("vmtr-viewer-sa")
				createClusterRoleBinding("vmtr-viewer-crb", viewerRole.Name, viewerSA)

				viewerClient := createClientForServiceAccount(viewerSA)
				err := createVirtualMachineTemplateRequest(viewerClient, "test-vmtr-viewer")
				Expect(err).To(HaveOccurred())
				Expect(k8serrors.IsForbidden(err)).To(BeTrue())
			})
		})
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
