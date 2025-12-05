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

package logs

// Log verbosity levels for use with log.V(level).
// Higher values = more verbose/debug output.
//
// With controller-runtime's zap integration, use the --zap-log-level flag:
//   - --zap-log-level=info  (or 0): Shows Info() calls only
//   - --zap-log-level=debug (or 1): Shows DebugLevel V() logs
//   - --zap-log-level=2:            Shows all V() logs up to TraceLevel
//
// Example: ./manager --zap-log-level=2 will show DebugLevel and TraceLevel.
const (
	DebugLevel = 1
	TraceLevel = 2
)
