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

// Package metricsauth lets the metrics endpoint authenticate the in-cluster Prometheus by its
// client certificate, as the OpenShift metrics conventions ask for.
package metricsauth

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"

	"github.com/go-logr/logr"
	"k8s.io/apiserver/pkg/authentication/request/x509"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

// debugLevel is the log level of rejected requests, as in the controller-runtime metrics filters.
const debugLevel = 4

// RequestClientCert makes the server ask for a client certificate without requiring one, so
// clients without a certificate can still use a bearer token.
func RequestClientCert(c *tls.Config) {
	c.ClientAuth = tls.RequestClientCert
}

// FilterProvider returns a metrics filter provider. Requests with a client certificate are
// verified against the CA from verifyOptions and allowed only for allowedCN, without calling the
// API server. Requests without a client certificate are passed to the filter of next. A client
// certificate that does not verify is rejected and never falls back to next.
func FilterProvider(verifyOptions x509.VerifyOptionFunc, allowedCN string,
	next func(*rest.Config, *http.Client) (metricsserver.Filter, error),
) func(*rest.Config, *http.Client) (metricsserver.Filter, error) {
	return filterProvider{
		certAuth:  x509.NewDynamic(verifyOptions, x509.CommonNameUserConversion),
		allowedCN: allowedCN,
		next:      next,
	}.provide
}

type filterProvider struct {
	certAuth  *x509.Authenticator
	allowedCN string
	next      func(*rest.Config, *http.Client) (metricsserver.Filter, error)
}

func (p filterProvider) provide(config *rest.Config, httpClient *http.Client) (metricsserver.Filter, error) {
	nextFilter, err := p.next(config, httpClient)
	if err != nil {
		return nil, err
	}

	return func(log logr.Logger, handler http.Handler) (http.Handler, error) {
		nextHandler, err := nextFilter(log, handler)
		if err != nil {
			return nil, err
		}
		return &certHandler{filterProvider: p, log: log, handler: handler, nextHandler: nextHandler}, nil
	}, nil
}

// certHandler serves requests with an allowed client certificate with handler and requests
// without one with nextHandler.
type certHandler struct {
	filterProvider
	log         logr.Logger
	handler     http.Handler
	nextHandler http.Handler
}

func (h *certHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.TLS == nil || len(req.TLS.PeerCertificates) == 0 {
		h.nextHandler.ServeHTTP(w, req)
		return
	}

	res, ok, err := h.certAuth.AuthenticateRequest(req)
	if !ok && err == nil {
		// Only happens before the client CA is loaded, the request has a certificate.
		err = errors.New("client CA is not loaded yet")
	}
	if err != nil {
		h.log.V(debugLevel).Info("Client certificate authentication failed", "error", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if res.User.GetName() != h.allowedCN {
		h.log.V(debugLevel).Info("Client certificate not allowed", "user", res.User.GetName())
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	h.handler.ServeHTTP(w, req)
}

// ClientCARunnable returns a runnable that calls run(ctx, 1) on every replica, not only on the
// leader, because the metrics server serves on all of them.
func ClientCARunnable(run func(ctx context.Context, workers int)) manager.Runnable {
	return clientCARunnable(run)
}

type clientCARunnable func(ctx context.Context, workers int)

func (r clientCARunnable) Start(ctx context.Context) error {
	r(ctx, 1)
	return nil
}

func (clientCARunnable) NeedLeaderElection() bool { return false }
