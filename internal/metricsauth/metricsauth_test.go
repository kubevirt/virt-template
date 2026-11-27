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

package metricsauth_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8sx509 "k8s.io/apiserver/pkg/authentication/request/x509"
	"k8s.io/apiserver/pkg/server/dynamiccertificates"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"kubevirt.io/virt-template/internal/metricsauth"
)

const allowedCN = "system:serviceaccount:openshift-monitoring:prometheus-k8s"

var _ = Describe("FilterProvider", func() {
	var (
		ca       *x509.Certificate
		caKey    *ecdsa.PrivateKey
		handler  http.Handler
		nextUsed bool
	)

	newHandler := func(verifyOptions k8sx509.VerifyOptionFunc) {
		nextUsed = false
		next := func(_ *rest.Config, _ *http.Client) (metricsserver.Filter, error) {
			return func(_ logr.Logger, _ http.Handler) (http.Handler, error) {
				return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					nextUsed = true
					w.WriteHeader(http.StatusTeapot)
				}), nil
			}, nil
		}

		filter, err := metricsauth.FilterProvider(verifyOptions, allowedCN, next)(nil, nil)
		Expect(err).ToNot(HaveOccurred())
		handler, err = filter(logr.Discard(), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		Expect(err).ToNot(HaveOccurred())
	}

	BeforeEach(func() {
		ca, caKey = newCert("test-ca", nil, nil)
		caProvider, err := dynamiccertificates.NewStaticCAContent("test",
			pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.Raw}))
		Expect(err).ToNot(HaveOccurred())
		newHandler(caProvider.VerifyOptions)
	})

	serve := func(certs ...*x509.Certificate) int {
		req := httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody)
		if len(certs) > 0 {
			req.TLS = &tls.ConnectionState{PeerCertificates: certs}
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}

	DescribeTable(
		"should authenticate by client certificate",
		func(cn string, foreignCA bool, expectedCode int, expectNext bool) {
			var certs []*x509.Certificate
			if cn != "" {
				signer, signerKey := ca, caKey
				if foreignCA {
					signer, signerKey = newCert("other-ca", nil, nil)
				}
				cert, _ := newCert(cn, signer, signerKey)
				certs = append(certs, cert)
			}
			Expect(serve(certs...)).To(Equal(expectedCode))
			Expect(nextUsed).To(Equal(expectNext))
		},
		Entry("allow the allowed common name without the next filter", allowedCN, false, http.StatusOK, false),
		Entry("forbid another common name", "system:serviceaccount:default:other", false, http.StatusForbidden, false),
		Entry("reject a certificate signed by another CA", allowedCN, true, http.StatusUnauthorized, false),
		Entry("pass requests without a client certificate to the next filter", "", false, http.StatusTeapot, true),
	)

	It("should reject a client certificate before the CA is loaded", func() {
		newHandler(func() (x509.VerifyOptions, bool) { return x509.VerifyOptions{}, false })
		cert, _ := newCert(allowedCN, ca, caKey)
		Expect(serve(cert)).To(Equal(http.StatusUnauthorized))
		Expect(nextUsed).To(BeFalse())
	})
})

var _ = Describe("ClientCARunnable", func() {
	It("should run on every replica, not only on the leader", func() {
		var workers int
		r := metricsauth.ClientCARunnable(func(_ context.Context, w int) { workers = w })

		ler, ok := r.(manager.LeaderElectionRunnable)
		Expect(ok).To(BeTrue())
		Expect(ler.NeedLeaderElection()).To(BeFalse())

		Expect(r.Start(context.Background())).To(Succeed())
		Expect(workers).To(Equal(1))
	})
})

var _ = Describe("RequestClientCert", func() {
	It("should request but not require a client certificate", func() {
		c := &tls.Config{}
		metricsauth.RequestClientCert(c)
		Expect(c.ClientAuth).To(Equal(tls.RequestClientCert))
	})
})

// newCert creates a certificate with the common name cn, signed by parent. Without a parent it
// creates a self-signed CA.
func newCert(cn string, parent *x509.Certificate, parentKey *ecdsa.PrivateKey) (*x509.Certificate, *ecdsa.PrivateKey) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	Expect(err).ToNot(HaveOccurred())

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	if parent == nil {
		tmpl.IsCA = true
		tmpl.BasicConstraintsValid = true
		tmpl.KeyUsage |= x509.KeyUsageCertSign
		parent, parentKey = tmpl, key
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, &key.PublicKey, parentKey)
	Expect(err).ToNot(HaveOccurred())
	cert, err := x509.ParseCertificate(der)
	Expect(err).ToNot(HaveOccurred())
	return cert, key
}
