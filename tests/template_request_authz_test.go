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
	const vmtrAuthzTest = "vmtr-authz-test-"

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

	createRoleBinding := func(namespace, saName, saNamespace, roleName string) *rbacv1.RoleBinding {
		rb := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    namespace,
				GenerateName: vmtrAuthzTest,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      saName,
					Namespace: saNamespace,
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     roleName,
			},
		}
		rb, err := virtClient.RbacV1().RoleBindings(namespace).Create(context.Background(), rb, metav1.CreateOptions{})
		ExpectWithOffset(1, err).ToNot(HaveOccurred())
		roleBindings = append(roleBindings, rb)
		return rb
	}

	createTemplateRequest := func() error {
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
					Namespace: NamespaceSecondaryTest,
					Name:      "test-vm",
				},
			},
		}
		tplReq, err = saClient.TemplateV1alpha1().VirtualMachineTemplateRequests(NamespaceTest).
			Create(context.Background(), tplReq, metav1.CreateOptions{})
		return err
	}

	Context("when user lacks permissions in source namespace", func() {
		BeforeEach(func() {
			role := createRole(NamespaceTest, []rbacv1.PolicyRule{
				{
					APIGroups: []string{templateapi.GroupName},
					Resources: []string{templateapi.PluralRequestResourceName},
					Verbs:     []string{"get", "list", "watch", "create"},
				},
				{
					APIGroups: []string{templateapi.GroupName},
					Resources: []string{templateapi.PluralResourceName},
					Verbs:     []string{"create"},
				},
				{
					APIGroups: []string{"cdi.kubevirt.io"},
					Resources: []string{"datavolumes"},
					Verbs:     []string{"create"},
				},
			})
			createRoleBinding(NamespaceTest, serviceAccount.Name, serviceAccount.Namespace, role.Name)
		})

		It("should deny when user cannot get VirtualMachine", func() {
			Expect(createTemplateRequest()).To(MatchError(ContainSubstring("User is not allowed to get VirtualMachines")))
		})

		It("should deny when user cannot create VirtualMachineSnapshots", func() {
			role := createRole(NamespaceSecondaryTest, []rbacv1.PolicyRule{
				{
					APIGroups: []string{"kubevirt.io"},
					Resources: []string{"virtualmachines"},
					Verbs:     []string{"get"},
				},
			})
			createRoleBinding(NamespaceSecondaryTest, serviceAccount.Name, serviceAccount.Namespace, role.Name)

			Expect(createTemplateRequest()).To(MatchError(ContainSubstring("User is not allowed to create VirtualMachineSnapshots")))
		})

		It("should deny when user cannot get VirtualMachineSnapshotContents", func() {
			role := createRole(NamespaceSecondaryTest, []rbacv1.PolicyRule{
				{
					APIGroups: []string{"kubevirt.io"},
					Resources: []string{"virtualmachines"},
					Verbs:     []string{"get"},
				},
				{
					APIGroups: []string{"snapshot.kubevirt.io"},
					Resources: []string{"virtualmachinesnapshots"},
					Verbs:     []string{"create"},
				},
			})
			createRoleBinding(NamespaceSecondaryTest, serviceAccount.Name, serviceAccount.Namespace, role.Name)

			Expect(createTemplateRequest()).To(MatchError(ContainSubstring("User is not allowed to get VirtualMachineSnapshotContents")))
		})

		It("should deny when user cannot expand VM spec", func() {
			role := createRole(NamespaceSecondaryTest, []rbacv1.PolicyRule{
				{
					APIGroups: []string{"kubevirt.io"},
					Resources: []string{"virtualmachines"},
					Verbs:     []string{"get"},
				},
				{
					APIGroups: []string{"snapshot.kubevirt.io"},
					Resources: []string{"virtualmachinesnapshots"},
					Verbs:     []string{"create"},
				},
				{
					APIGroups: []string{"snapshot.kubevirt.io"},
					Resources: []string{"virtualmachinesnapshotcontents"},
					Verbs:     []string{"get"},
				},
			})
			createRoleBinding(NamespaceSecondaryTest, serviceAccount.Name, serviceAccount.Namespace, role.Name)

			Expect(createTemplateRequest()).To(MatchError(ContainSubstring("User is not allowed to expand VM spec")))
		})
	})

	Context("when user lacks permissions in request namespace", func() {
		BeforeEach(func() {
			role := createRole(NamespaceSecondaryTest, []rbacv1.PolicyRule{
				{
					APIGroups: []string{"kubevirt.io"},
					Resources: []string{"virtualmachines"},
					Verbs:     []string{"get"},
				},
				{
					APIGroups: []string{"snapshot.kubevirt.io"},
					Resources: []string{"virtualmachinesnapshots"},
					Verbs:     []string{"create"},
				},
				{
					APIGroups: []string{"snapshot.kubevirt.io"},
					Resources: []string{"virtualmachinesnapshotcontents"},
					Verbs:     []string{"get"},
				},
				{
					APIGroups: []string{"subresources.kubevirt.io"},
					Resources: []string{"expand-vm-spec"},
					Verbs:     []string{"update"},
				},
			})
			createRoleBinding(NamespaceSecondaryTest, serviceAccount.Name, serviceAccount.Namespace, role.Name)
		})

		It("should deny when user cannot create DataVolumes", func() {
			role := createRole(NamespaceTest, []rbacv1.PolicyRule{
				{
					APIGroups: []string{templateapi.GroupName},
					Resources: []string{templateapi.PluralRequestResourceName},
					Verbs:     []string{"create", "get", "list", "watch"},
				},
				{
					APIGroups: []string{templateapi.GroupName},
					Resources: []string{templateapi.PluralResourceName},
					Verbs:     []string{"create"},
				},
			})
			createRoleBinding(NamespaceTest, serviceAccount.Name, serviceAccount.Namespace, role.Name)

			Expect(createTemplateRequest()).To(MatchError(ContainSubstring("User is not allowed to create DataVolumes")))
		})

		It("should deny when user cannot create VirtualMachineTemplates", func() {
			role := createRole(NamespaceTest, []rbacv1.PolicyRule{
				{
					APIGroups: []string{templateapi.GroupName},
					Resources: []string{templateapi.PluralRequestResourceName},
					Verbs:     []string{"create", "get", "list", "watch"},
				},
				{
					APIGroups: []string{"cdi.kubevirt.io"},
					Resources: []string{"datavolumes"},
					Verbs:     []string{"create"},
				},
			})
			createRoleBinding(NamespaceTest, serviceAccount.Name, serviceAccount.Namespace, role.Name)

			Expect(createTemplateRequest()).To(MatchError(ContainSubstring("User is not allowed to create VirtualMachineTemplates")))
		})
	})

	It("when user has all required permissions it should allow creating VirtualMachineTemplateRequest", func() {
		sourceRole := createRole(NamespaceSecondaryTest, []rbacv1.PolicyRule{
			{
				APIGroups: []string{"kubevirt.io"},
				Resources: []string{"virtualmachines"},
				Verbs:     []string{"get"},
			},
			{
				APIGroups: []string{"snapshot.kubevirt.io"},
				Resources: []string{"virtualmachinesnapshots"},
				Verbs:     []string{"create"},
			},
			{
				APIGroups: []string{"snapshot.kubevirt.io"},
				Resources: []string{"virtualmachinesnapshotcontents"},
				Verbs:     []string{"get"},
			},
			{
				APIGroups: []string{"subresources.kubevirt.io"},
				Resources: []string{"expand-vm-spec"},
				Verbs:     []string{"update"},
			},
		})
		createRoleBinding(NamespaceSecondaryTest, serviceAccount.Name, serviceAccount.Namespace, sourceRole.Name)

		requestRole := createRole(NamespaceTest, []rbacv1.PolicyRule{
			{
				APIGroups: []string{templateapi.GroupName},
				Resources: []string{templateapi.PluralRequestResourceName},
				Verbs:     []string{"create", "get", "list", "watch"},
			},
			{
				APIGroups: []string{templateapi.GroupName},
				Resources: []string{templateapi.PluralResourceName},
				Verbs:     []string{"create"},
			},
			{
				APIGroups: []string{"cdi.kubevirt.io"},
				Resources: []string{"datavolumes"},
				Verbs:     []string{"create"},
			},
		})
		createRoleBinding(NamespaceTest, serviceAccount.Name, serviceAccount.Namespace, requestRole.Name)

		Expect(createTemplateRequest()).To(Succeed())
		Expect(tplReq.Name).ToNot(BeEmpty())
	})
})
