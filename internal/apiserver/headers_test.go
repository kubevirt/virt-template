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
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = DescribeTable(
	"withSecureResponseHeaders",
	func(
		innerHandler func(w http.ResponseWriter),
		expectedStatus int,
		expectedContentTypeOptions, expectedCacheControl string,
	) {
		handler := withSecureResponseHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			innerHandler(w)
		}))

		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", http.NoBody))

		result := recorder.Result()
		Expect(result.StatusCode).To(Equal(expectedStatus))
		Expect(result.Header.Values(xContentTypeOptions)).To(ConsistOf(expectedContentTypeOptions))
		Expect(result.Header.Get(cacheControl)).To(Equal(expectedCacheControl))
	},
	Entry(
		"appends all required directives when the wrapped handler sets no Cache-Control",
		func(w http.ResponseWriter) { _, _ = w.Write([]byte("ok")) },
		http.StatusOK,
		"nosniff",
		"no-cache, no-store, must-revalidate",
	),
	Entry(
		"keeps a X-Content-Type-Options value that already contains nosniff",
		func(w http.ResponseWriter) {
			w.Header().Set(xContentTypeOptions, "NOSNIFF")
			w.WriteHeader(http.StatusOK)
		},
		http.StatusOK,
		"NOSNIFF",
		"no-cache, no-store, must-revalidate",
	),
	Entry(
		"overrides a X-Content-Type-Options value that doesn't contain nosniff",
		func(w http.ResponseWriter) {
			w.Header().Set(xContentTypeOptions, "custom-value")
			w.WriteHeader(http.StatusOK)
		},
		http.StatusOK,
		"nosniff",
		"no-cache, no-store, must-revalidate",
	),
	Entry(
		"appends only the missing directives to an existing Cache-Control value",
		func(w http.ResponseWriter) {
			w.Header().Set(cacheControl, "no-cache, private")
			w.WriteHeader(http.StatusOK)
		},
		http.StatusOK,
		"nosniff",
		"no-cache, private, no-store, must-revalidate",
	),
	Entry(
		"does not append anything when all required directives are already present",
		func(w http.ResponseWriter) {
			w.Header().Set(cacheControl, "no-cache, no-store, must-revalidate, private")
			w.WriteHeader(http.StatusOK)
		},
		http.StatusOK,
		"nosniff",
		"no-cache, no-store, must-revalidate, private",
	),
	Entry(
		"leaves a publicly cacheable Cache-Control value untouched",
		func(w http.ResponseWriter) {
			w.Header().Set(cacheControl, "public, immutable")
			w.WriteHeader(http.StatusOK)
		},
		http.StatusOK,
		"nosniff",
		"public, immutable",
	),
	Entry(
		"sets nosniff exactly once when WriteHeader is called explicitly before Write",
		func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte("created"))
		},
		http.StatusCreated,
		"nosniff",
		"no-cache, no-store, must-revalidate",
	),
	Entry(
		"sets headers when the handler returns without calling Write or WriteHeader",
		func(_ http.ResponseWriter) {},
		http.StatusOK,
		"nosniff",
		"no-cache, no-store, must-revalidate",
	),
	Entry(
		"leaves a mixed-case publicly cacheable Cache-Control value untouched",
		func(w http.ResponseWriter) {
			w.Header().Set(cacheControl, "Public, immutable")
			w.WriteHeader(http.StatusOK)
		},
		http.StatusOK,
		"nosniff",
		"Public, immutable",
	),
	Entry(
		"leaves Cache-Control untouched on a response that carries an Etag but no explicit Cache-Control",
		func(w http.ResponseWriter) {
			w.Header().Set(etag, `"abc123"`)
			w.WriteHeader(http.StatusOK)
		},
		http.StatusOK,
		"nosniff",
		"",
	),
)

// flushCloseNotifierWriter is an inner http.ResponseWriter that implements
// http.Flusher and http.CloseNotifier so that WrapForHTTP1Or2 takes its
// delegator path (the HTTP/2 case), exercising secureHeaderResponseWriter.Flush.
type flushCloseNotifierWriter struct {
	header  http.Header
	flushed bool
}

func (w *flushCloseNotifierWriter) Header() http.Header         { return w.header }
func (w *flushCloseNotifierWriter) WriteHeader(int)             {}
func (w *flushCloseNotifierWriter) Write(b []byte) (int, error) { return len(b), nil }
func (w *flushCloseNotifierWriter) Flush()                      { w.flushed = true }
func (w *flushCloseNotifierWriter) CloseNotify() <-chan bool    { return nil }

var _ = Describe("withSecureResponseHeaders", func() {
	It("sets secure headers and flushes through the delegator path", func() {
		inner := &flushCloseNotifierWriter{header: http.Header{}}
		handler := withSecureResponseHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			flusher, ok := w.(http.Flusher)
			Expect(ok).To(BeTrue())
			flusher.Flush()
		}))

		handler.ServeHTTP(inner, httptest.NewRequest(http.MethodGet, "/", http.NoBody))

		Expect(inner.flushed).To(BeTrue())
		Expect(inner.header.Get(xContentTypeOptions)).To(Equal("nosniff"))
		Expect(inner.header.Get(cacheControl)).To(Equal("no-cache, no-store, must-revalidate"))
	})
})

// hijackableWriter is an inner http.ResponseWriter that implements
// http.Hijacker so that Hijack calls can be exercised through the decorator.
type hijackableWriter struct {
	header   http.Header
	hijacked bool
}

func (w *hijackableWriter) Header() http.Header         { return w.header }
func (w *hijackableWriter) WriteHeader(int)             {}
func (w *hijackableWriter) Write(b []byte) (int, error) { return len(b), nil }

func (w *hijackableWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.hijacked = true
	server, client := net.Pipe()
	_ = client.Close()
	return server, bufio.NewReadWriter(bufio.NewReader(server), bufio.NewWriter(server)), nil
}

var _ = Describe("secureHeaderResponseWriter.Hijack", func() {
	It("marks the header as written instead of writing it after the connection is hijacked", func() {
		inner := &hijackableWriter{header: http.Header{}}
		handler := withSecureResponseHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			hijacker, ok := w.(http.Hijacker)
			Expect(ok).To(BeTrue())
			conn, _, err := hijacker.Hijack()
			Expect(err).ToNot(HaveOccurred())
			defer conn.Close()
		}))

		handler.ServeHTTP(inner, httptest.NewRequest(http.MethodGet, "/", http.NoBody))

		Expect(inner.hijacked).To(BeTrue())
		Expect(inner.header.Get(xContentTypeOptions)).To(BeEmpty())
	})
})
