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

package controller

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/discovery"
	snapshotv1beta1 "kubevirt.io/api/snapshot/v1beta1"
	"kubevirt.io/client-go/kubecli"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"kubevirt.io/virt-template-api/core/v1beta1"
)

const defaultPollInterval = 1 * time.Minute

var (
	dataVolumesResource      = cdiv1beta1.SchemeGroupVersion.WithResource("datavolumes")
	snapshotsResource        = snapshotv1beta1.SchemeGroupVersion.WithResource("virtualmachinesnapshots")
	snapshotContentsResource = snapshotv1beta1.SchemeGroupVersion.WithResource("virtualmachinesnapshotcontents")

	// CRDs of a group are registered one by one, so each resource is checked, not the group.
	requiredResources = []schema.GroupVersionResource{
		dataVolumesResource,
		snapshotsResource,
		snapshotContentsResource,
	}
)

// VMTRAvailabilityController polls for required external CRDs and
// starts the VMTR controller once all are available. While waiting,
// it sets status conditions on pending VirtualMachineTemplateRequests
// indicating which CRDs are missing.
type VMTRAvailabilityController struct {
	Manager         ctrl.Manager
	VirtClient      kubecli.KubevirtClient
	DiscoveryClient discovery.DiscoveryInterface

	pollInterval time.Duration
}

// SetPollInterval overrides the default CRD poll interval. Intended for tests.
func (c *VMTRAvailabilityController) SetPollInterval(d time.Duration) {
	c.pollInterval = d
}

func (c *VMTRAvailabilityController) Start(ctx context.Context) error {
	missing := c.missingResources()
	if len(missing) == 0 {
		return c.startController(ctx)
	}

	if _, err := c.Manager.GetCache().GetInformer(ctx, &v1beta1.VirtualMachineTemplateRequest{}); err != nil {
		return fmt.Errorf("failed to add VMTR informer: %w", err)
	}

	logf.FromContext(ctx).Info("Waiting for required CRDs to become available", "missing", missing)

	if c.pollInterval == 0 {
		c.SetPollInterval(defaultPollInterval)
	}
	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()

	for {
		missing = c.missingResources()
		if len(missing) == 0 {
			return c.startController(ctx)
		}

		c.setMissingCRDStatus(ctx, missing)

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (c *VMTRAvailabilityController) startController(ctx context.Context) error {
	logf.FromContext(ctx).Info("All required CRDs are available, starting controller")
	return (&VirtualMachineTemplateRequestReconciler{
		Client:     c.Manager.GetClient(),
		VirtClient: c.VirtClient,
		Scheme:     c.Manager.GetScheme(),
	}).SetupWithManager(c.Manager)
}

func (c *VMTRAvailabilityController) setMissingCRDStatus(ctx context.Context, missing []string) {
	log := logf.FromContext(ctx)
	cl := c.Manager.GetClient()

	list := &v1beta1.VirtualMachineTemplateRequestList{}
	if err := cl.List(ctx, list); err != nil {
		log.Error(err, "Failed to list VirtualMachineTemplateRequests")
		return
	}

	message := fmt.Sprintf("Required CRDs are not yet available: %s", strings.Join(missing, ", "))
	for i := range list.Items {
		tplReq := &list.Items[i]

		if !tplReq.DeletionTimestamp.IsZero() || !shouldReconcile(tplReq) {
			continue
		}

		tplReqCopy := tplReq.DeepCopy()
		setReadyCondition(ctx, tplReq, metav1.ConditionFalse, v1beta1.ReasonWaiting, "%s", message)
		setProgressingCondition(ctx, tplReq, metav1.ConditionTrue, v1beta1.ReasonWaiting)
		if equality.Semantic.DeepEqual(tplReq.Status, tplReqCopy.Status) {
			continue
		}

		if err := cl.Status().Patch(ctx, tplReq, client.MergeFrom(tplReqCopy)); err != nil {
			log.Error(err, "Failed to update VirtualMachineTemplateRequest status",
				"namespace", tplReq.Namespace, "name", tplReq.Name)
		}
	}
}

func (c *VMTRAvailabilityController) missingResources() []string {
	var missing []string
	for _, resource := range requiredResources {
		if !isResourceServed(c.DiscoveryClient, resource) {
			missing = append(missing, resource.GroupResource().String())
		}
	}
	return missing
}

func isResourceServed(dc discovery.DiscoveryInterface, resource schema.GroupVersionResource) bool {
	list, err := dc.ServerResourcesForGroupVersion(resource.GroupVersion().String())
	if err != nil {
		return false
	}
	return slices.ContainsFunc(list.APIResources, func(r metav1.APIResource) bool {
		return r.Name == resource.Resource
	})
}

// ExternalCRDCacheConfig only adds entries for served resources, since a cache.ByObject for a
// missing CRD fails manager construction.
func ExternalCRDCacheConfig(dc discovery.DiscoveryInterface) (map[client.Object]cache.ByObject, []client.Object) {
	uidReq, _ := labels.NewRequirement(v1beta1.LabelRequestUID, selection.Exists, nil)
	uidSelector := labels.NewSelector().Add(*uidReq)

	cacheByObject := map[client.Object]cache.ByObject{}
	var clientDisableFor []client.Object

	if isResourceServed(dc, dataVolumesResource) {
		cacheByObject[&cdiv1beta1.DataVolume{}] = cache.ByObject{Label: uidSelector}
	}
	if isResourceServed(dc, snapshotsResource) {
		cacheByObject[&snapshotv1beta1.VirtualMachineSnapshot{}] = cache.ByObject{Label: uidSelector}
	}
	if isResourceServed(dc, snapshotContentsResource) {
		clientDisableFor = append(clientDisableFor, &snapshotv1beta1.VirtualMachineSnapshotContent{})
	}

	return cacheByObject, clientDisableFor
}
