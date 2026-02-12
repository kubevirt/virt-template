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

	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/yaml"
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

	// All resource verbs we assert; tests define expected allowed subset per role.
	allVerbs := []string{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"}

	checkPermission := func(sa *corev1.ServiceAccount, apiGroup, resource, verb string) bool {
		sar := &authorizationv1.SubjectAccessReview{
			Spec: authorizationv1.SubjectAccessReviewSpec{
				User: "system:serviceaccount:" + sa.Namespace + ":" + sa.Name,
				ResourceAttributes: &authorizationv1.ResourceAttributes{
					Group:     apiGroup,
					Resource:  resource,
					Verb:      verb,
					Namespace: testNamespace,
				},
			},
		}
		result, err := k8sClient.AuthorizationV1().SubjectAccessReviews().Create(context.Background(), sar, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		return result.Status.Allowed
	}

	testRBACPermissions := func(roleName, resourceName, apiGroup string, expectedAllowedVerbs []string) {
		role := clusterRoles[roleName]
		Expect(role).NotTo(BeNil())

		sa := createServiceAccount(roleName + "-sa")
		createClusterRoleBinding(roleName+"-crb", role.Name, sa)

		expectedAllowed := sets.New(expectedAllowedVerbs...)
		for _, verb := range allVerbs {
			expected := expectedAllowed.Has(verb)
			actual := checkPermission(sa, apiGroup, resourceName, verb)
			Expect(actual).To(Equal(expected),
				"Role %s should %s have %s permission on %s",
				roleName, map[bool]string{true: "", false: "not"}[expected], verb, resourceName)
		}
	}

	adminVerbs := []string{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"}
	editorVerbs := []string{"create", "delete", "get", "list", "patch", "update", "watch"}
	viewerVerbs := []string{"get", "list", "watch"}

	Context("VirtualMachineTemplate roles", func() {
		DescribeTable("RBAC permissions", testRBACPermissions,
			Entry("Admin role", "virtualmachinetemplate-admin-role", "virtualmachinetemplates", "template.kubevirt.io", adminVerbs),
			Entry("Editor role", "virtualmachinetemplate-editor-role", "virtualmachinetemplates", "template.kubevirt.io", editorVerbs),
			Entry("Viewer role", "virtualmachinetemplate-viewer-role", "virtualmachinetemplates", "template.kubevirt.io", viewerVerbs),
		)
	})

	Context("VirtualMachineTemplateRequest roles", func() {
		DescribeTable("RBAC permissions", testRBACPermissions,
			Entry("Admin role", "virtualmachinetemplaterequest-admin-role", "virtualmachinetemplaterequests", "template.kubevirt.io", adminVerbs),
			Entry("Editor role", "virtualmachinetemplaterequest-editor-role", "virtualmachinetemplaterequests", "template.kubevirt.io", editorVerbs),
			Entry("Viewer role", "virtualmachinetemplaterequest-viewer-role", "virtualmachinetemplaterequests", "template.kubevirt.io", viewerVerbs),
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
