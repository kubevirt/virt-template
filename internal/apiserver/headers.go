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
	"errors"
	"net"
	"net/http"
	"slices"
	"strings"

	"k8s.io/apiserver/pkg/endpoints/responsewriter"
)

const (
	xContentTypeOptions = "X-Content-Type-Options"
	cacheControl        = "Cache-Control"
	etag                = "Etag"
)

// requiredCacheControlDirectives are the directives OWASP recommends for
// authenticated content. See
// https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html#web-content-caching
var requiredCacheControlDirectives = []string{"no-cache", "no-store", "must-revalidate"}

// withSecureResponseHeaders ensures X-Content-Type-Options contains
// "nosniff" and Cache-Control carries requiredCacheControlDirectives on
// every response, matching what ZAP's passive scan rules check for. The
// generic apiserver handler chain only sets these for a handful of paths.
func withSecureResponseHeaders(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		sw := &secureHeaderResponseWriter{ResponseWriter: w}
		handler.ServeHTTP(responsewriter.WrapForHTTP1Or2(sw), req)
		sw.ensureHeaderWritten()
	})
}

type secureHeaderResponseWriter struct {
	http.ResponseWriter
	wroteHeader bool
}

func (w *secureHeaderResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// Hijack marks the header as written so that ensureHeaderWritten does not
// call WriteHeader after the connection has been taken over by the caller.
func (w *secureHeaderResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.wroteHeader = true
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("underlying ResponseWriter does not implement http.Hijacker")
	}
	return hijacker.Hijack()
}

func (w *secureHeaderResponseWriter) WriteHeader(statusCode int) {
	if !w.wroteHeader {
		w.wroteHeader = true
		header := w.ResponseWriter.Header()
		if !strings.Contains(strings.ToLower(header.Get(xContentTypeOptions)), "nosniff") {
			header.Set(xContentTypeOptions, "nosniff")
		}
		ensureSecureCacheControl(header)
	}
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *secureHeaderResponseWriter) Write(b []byte) (int, error) {
	w.ensureHeaderWritten()
	return w.ResponseWriter.Write(b)
}

func (w *secureHeaderResponseWriter) Flush() {
	w.ensureHeaderWritten()
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *secureHeaderResponseWriter) ensureHeaderWritten() {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
}

// ensureSecureCacheControl appends missing requiredCacheControlDirectives.
// A "public" directive (e.g. kube-openapi's hash-addressed, immutable OpenAPI
// documents) is left untouched, since no-store/must-revalidate would
// contradict deliberate, shared caching. Responses that carry an Etag (e.g.
// kube-openapi's unhashed discovery documents) are left untouched too, since
// no-store would defeat the conditional-request/304 mechanism they rely on.
func ensureSecureCacheControl(header http.Header) {
	if header.Get(etag) != "" {
		return
	}

	current := header.Get(cacheControl)

	var directives []string
	if current != "" {
		for _, directive := range strings.Split(current, ",") {
			directives = append(directives, strings.TrimSpace(directive))
		}
	}

	if slices.ContainsFunc(directives, func(d string) bool { return strings.EqualFold(d, "public") }) {
		return
	}

	for _, required := range requiredCacheControlDirectives {
		if !slices.ContainsFunc(directives, func(d string) bool { return strings.EqualFold(d, required) }) {
			directives = append(directives, required)
		}
	}
	header.Set(cacheControl, strings.Join(directives, ", "))
}
