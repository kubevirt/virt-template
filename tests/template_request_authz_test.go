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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	templateapi "kubevirt.io/virt-template-api/core"
	"kubevirt.io/virt-template-api/core/v1alpha1"
	templateclient "kubevirt.io/virt-template-client-go/virttemplate"
)

var _ = Describe("VirtualMachineTemplateRequest Authorization", func() {
	const (
		vmtrAuthzTest     = "vmtr-authz-test-"
		roleKind          = "Role"
		clusterRoleKind   = "ClusterRole"
		sourceRoleName    = "virt-template-virtualmachinetemplaterequest-source-role"
		sourceSubresource = templateapi.PluralRequestResourceName + "/source"
		testVMName        = "test-vm"
		verbCreate        = "create"
	)

	var (
		serviceAccount *corev1.ServiceAccount
		roles          []*rbacv1.Role
		roleBindings   []*rbacv1.RoleBinding
		tplReq         *v1alpha1.VirtualMachineTemplateRequest
	)

	BeforeEach(func() {
		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    NamespaceTest,
				GenerateName: vmtrAuthzTest,
			},
		}
		var err error
		serviceAccount, err = virtClient.CoreV1().ServiceAccounts(NamespaceTest).
			Create(context.Background(), serviceAccount, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		if serviceAccount != nil {
			Expect(virtClient.CoreV1().ServiceAccounts(NamespaceTest).Delete(context.Background(), serviceAccount.Name, metav1.DeleteOptions{})).
				To(Or(Succeed(), MatchError(k8serrors.IsNotFound, "k8serrors.IsNotFound")))
		}
		if tplReq != nil && tplReq.Name != "" {
			Expect(tplClient.TemplateV1alpha1().VirtualMachineTemplateRequests(NamespaceTest).
				Delete(context.Background(), tplReq.Name, metav1.DeleteOptions{})).
				To(Or(Succeed(), MatchError(k8serrors.IsNotFound, "k8serrors.IsNotFound")))
		}
		for _, role := range roles {
			Expect(virtClient.RbacV1().Roles(role.Namespace).Delete(context.Background(), role.Name, metav1.DeleteOptions{})).
				To(Or(Succeed(), MatchError(k8serrors.IsNotFound, "k8serrors.IsNotFound")))
		}
		for _, roleBinding := range roleBindings {
			Expect(virtClient.RbacV1().RoleBindings(roleBinding.Namespace).Delete(context.Background(), roleBinding.Name, metav1.DeleteOptions{})).
				To(Or(Succeed(), MatchError(k8serrors.IsNotFound, "k8serrors.IsNotFound")))
		}
	})

	createRole := func(namespace string, rules []rbacv1.PolicyRule) *rbacv1.Role {
		role := &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    namespace,
				GenerateName: vmtrAuthzTest,
			},
			Rules: rules,
		}
		role, err := virtClient.RbacV1().Roles(namespace).Create(context.Background(), role, metav1.CreateOptions{})
		ExpectWithOffset(1, err).ToNot(HaveOccurred())
		roles = append(roles, role)
		return role
	}

	createRoleBinding := func(namespace, roleKind, roleName string) {
		rb := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    namespace,
				GenerateName: vmtrAuthzTest,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      serviceAccount.Name,
					Namespace: serviceAccount.Namespace,
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     roleKind,
				Name:     roleName,
			},
		}
		rb, err := virtClient.RbacV1().RoleBindings(namespace).Create(context.Background(), rb, metav1.CreateOptions{})
		ExpectWithOffset(1, err).ToNot(HaveOccurred())
		roleBindings = append(roleBindings, rb)
	}

	createTemplateRequest := func(sourceNamespace string) error {
		cfg := rest.CopyConfig(virtClient.Config())
		cfg.Impersonate = rest.ImpersonationConfig{
			UserName: "system:serviceaccount:" + serviceAccount.Namespace + ":" + serviceAccount.Name,
		}
		saClient, err := templateclient.NewForConfig(cfg)
		ExpectWithOffset(1, err).ToNot(HaveOccurred())

		tplReq = &v1alpha1.VirtualMachineTemplateRequest{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: vmtrAuthzTest,
				Namespace:    NamespaceTest,
			},
			Spec: v1alpha1.VirtualMachineTemplateRequestSpec{
				VirtualMachineRef: v1alpha1.VirtualMachineReference{
					Namespace: sourceNamespace,
					Name:      testVMName,
				},
			},
		}
		tplReq, err = saClient.TemplateV1alpha1().VirtualMachineTemplateRequests(NamespaceTest).
			Create(context.Background(), tplReq, metav1.CreateOptions{})
		return err
	}

	Context("when user lacks source permissions for cross namespace clone", func() {
		BeforeEach(func() {
			requestRole := createRole(NamespaceTest, []rbacv1.PolicyRule{
				{
					APIGroups: []string{templateapi.GroupName},
					Resources: []string{templateapi.PluralRequestResourceName},
					Verbs:     []string{verbCreate},
				},
			})
			createRoleBinding(NamespaceTest, roleKind, requestRole.Name)
		})

		It("should deny when user has no source role", func() {
			Expect(createTemplateRequest(NamespaceSecondaryTest)).To(MatchError(ContainSubstring("User is not allowed to use VirtualMachine")))
		})

		It("should deny when source role restricts to a different resourceName", func() {
			sourceRole := createRole(NamespaceSecondaryTest, []rbacv1.PolicyRule{
				{
					APIGroups:     []string{templateapi.GroupName},
					Resources:     []string{sourceSubresource},
					ResourceNames: []string{"other-vm"},
					Verbs:         []string{verbCreate},
				},
			})
			createRoleBinding(NamespaceSecondaryTest, roleKind, sourceRole.Name)

			Expect(createTemplateRequest(NamespaceSecondaryTest)).To(MatchError(ContainSubstring("User is not allowed to use VirtualMachine")))
		})
	})

	Context("when user lacks permissions in request namespace", func() {
		BeforeEach(func() {
			createRoleBinding(NamespaceSecondaryTest, clusterRoleKind, sourceRoleName)
		})

		It("should deny when user cannot create DataVolumes", func() {
			role := createRole(NamespaceTest, []rbacv1.PolicyRule{
				{
					APIGroups: []string{templateapi.GroupName},
					Resources: []string{templateapi.PluralRequestResourceName},
					Verbs:     []string{verbCreate},
				},
				{
					APIGroups: []string{templateapi.GroupName},
					Resources: []string{templateapi.PluralResourceName},
					Verbs:     []string{verbCreate},
				},
			})
			createRoleBinding(NamespaceTest, roleKind, role.Name)

			Expect(createTemplateRequest(NamespaceSecondaryTest)).To(MatchError(ContainSubstring("User is not allowed to create DataVolumes")))
		})

		It("should deny when user cannot create VirtualMachineTemplates", func() {
			role := createRole(NamespaceTest, []rbacv1.PolicyRule{
				{
					APIGroups: []string{templateapi.GroupName},
					Resources: []string{templateapi.PluralRequestResourceName},
					Verbs:     []string{verbCreate},
				},
				{
					APIGroups: []string{"cdi.kubevirt.io"},
					Resources: []string{"datavolumes"},
					Verbs:     []string{verbCreate},
				},
			})
			createRoleBinding(NamespaceTest, roleKind, role.Name)

			Expect(createTemplateRequest(NamespaceSecondaryTest)).
				To(MatchError(ContainSubstring("User is not allowed to create VirtualMachineTemplates")))
		})
	})

	Context("when user has all required permissions", func() {
		BeforeEach(func() {
			requestRole := createRole(NamespaceTest, []rbacv1.PolicyRule{
				{
					APIGroups: []string{templateapi.GroupName},
					Resources: []string{templateapi.PluralRequestResourceName},
					Verbs:     []string{verbCreate},
				},
				{
					APIGroups: []string{templateapi.GroupName},
					Resources: []string{templateapi.PluralResourceName},
					Verbs:     []string{verbCreate},
				},
				{
					APIGroups: []string{"cdi.kubevirt.io"},
					Resources: []string{"datavolumes"},
					Verbs:     []string{verbCreate},
				},
			})
			createRoleBinding(NamespaceTest, roleKind, requestRole.Name)
		})

		It("should allow when source role restricts to the referenced resourceName", func() {
			sourceRole := createRole(NamespaceSecondaryTest, []rbacv1.PolicyRule{
				{
					APIGroups:     []string{templateapi.GroupName},
					Resources:     []string{sourceSubresource},
					ResourceNames: []string{testVMName},
					Verbs:         []string{verbCreate},
				},
			})
			createRoleBinding(NamespaceSecondaryTest, roleKind, sourceRole.Name)

			Expect(createTemplateRequest(NamespaceSecondaryTest)).To(Succeed())
			Expect(tplReq.Name).ToNot(BeEmpty())
		})

		It("should allow when source role does not restrict resourceName", func() {
			createRoleBinding(NamespaceSecondaryTest, clusterRoleKind, sourceRoleName)

			Expect(createTemplateRequest(NamespaceSecondaryTest)).To(Succeed())
			Expect(tplReq.Name).ToNot(BeEmpty())
		})

		It("should allow same namespace clone without source role", func() {
			Expect(createTemplateRequest(NamespaceTest)).To(Succeed())
			Expect(tplReq.Name).ToNot(BeEmpty())
		})
	})
})
