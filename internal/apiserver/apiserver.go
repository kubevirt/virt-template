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

package apiserver

import (
	"context"
	"flag"
	"os/signal"
	"syscall"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/util/compatibility"
	"k8s.io/klog/v2"
	openapicommon "k8s.io/kube-openapi/pkg/common"
)

type (
	APIGroups = map[schema.GroupVersion]map[string]rest.Storage
	apiserver struct {
		secureServingOpts *options.SecureServingOptionsWithLoopback
		authnOpts         *options.DelegatingAuthenticationOptions
		authzOpts         *options.DelegatingAuthorizationOptions
	}
)

func New() *apiserver {
	return &apiserver{
		secureServingOpts: options.NewSecureServingOptions().WithLoopback(),
		authnOpts:         options.NewDelegatingAuthenticationOptions(),
		authzOpts:         options.NewDelegatingAuthorizationOptions(),
	}
}

func (a *apiserver) AddFlags(fs *pflag.FlagSet) {
	a.secureServingOpts.AddFlags(fs)
	a.authnOpts.AddFlags(fs)
	a.authzOpts.AddFlags(fs)

	goFs := flag.NewFlagSet("", flag.ExitOnError)
	klog.InitFlags(goFs)
	fs.AddGoFlagSet(goFs)
}

func (a *apiserver) Run(
	name string,
	scheme *runtime.Scheme,
	openAPIConfig *openapicommon.Config,
	openapiV3Config *openapicommon.OpenAPIV3Config,
	apiGroups APIGroups,
) error {
	factory := serializer.NewCodecFactory(scheme)
	config := genericapiserver.NewRecommendedConfig(factory)
	config.EffectiveVersion = compatibility.DefaultBuildEffectiveVersion()
	config.OpenAPIConfig = openAPIConfig
	config.OpenAPIV3Config = openapiV3Config
	// Disable discovery to not confuse kubectl and other client with dummy resources
	config.EnableDiscovery = false

	a.authzOpts.AlwaysAllowPaths = append(a.authzOpts.AlwaysAllowPaths,
		"/", genericapiserver.APIGroupPrefix, "/openapi/v2", "/openapi/v3", "/openapi/v3/*",
	)
	a.authzOpts.AlwaysAllowPaths = append(a.authzOpts.AlwaysAllowPaths,
		getAdditionalAlwaysAllowPaths(apiGroups)...,
	)

	if err := a.secureServingOpts.ApplyTo(&config.SecureServing, &config.LoopbackClientConfig); err != nil {
		klog.Errorf("Failed to apply secure serving options: %v", err)
		return err
	}
	if err := a.authnOpts.ApplyTo(&config.Authentication, config.SecureServing, config.OpenAPIConfig); err != nil {
		klog.Errorf("Failed to apply authentication options: %v", err)
		return err
	}
	if err := a.authzOpts.ApplyTo(&config.Authorization); err != nil {
		klog.Errorf("Failed to apply authorization options: %v", err)
		return err
	}

	server, err := config.Complete().New(name, genericapiserver.NewEmptyDelegate())
	if err != nil {
		klog.Errorf("Failed to create server: %v", err)
		return err
	}

	for gv, resourcesStorage := range apiGroups {
		groupInfo := genericapiserver.NewDefaultAPIGroupInfo(
			gv.Group, scheme, runtime.NewParameterCodec(scheme), factory,
		)
		groupInfo.VersionedResourcesStorageMap[gv.Version] = resourcesStorage
		if err := server.InstallAPIGroup(&groupInfo); err != nil {
			klog.Errorf("Failed to install APIGroup: %v", err)
			return err
		}
		resourcesToHide := getParentResourceNames(resourcesStorage)
		if len(resourcesToHide) > 0 {
			klog.Infof("Hiding parent resources from APIResourceList: %v", resourcesToHide)
			if err := installFilteredAPIVersionHandler(gv, resourcesToHide, server.Handler.GoRestfulContainer, factory); err != nil {
				return err
			}
		}
	}

	signalsCtx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	klog.Info("Starting aggregated API server...")
	if err := server.PrepareRun().RunWithContext(signalsCtx); err != nil {
		klog.Errorf("Failed to run server: %v", err)
		return err
	}

	return nil
}

func getAdditionalAlwaysAllowPaths(apiGroups APIGroups) []string {
	var additionalAlwaysAllowPaths []string
	for gv := range apiGroups {
		additionalAlwaysAllowPaths = append(additionalAlwaysAllowPaths,
			genericapiserver.APIGroupPrefix+"/"+gv.Group,
			genericapiserver.APIGroupPrefix+"/"+gv.String(),
		)
	}
	return additionalAlwaysAllowPaths
}
