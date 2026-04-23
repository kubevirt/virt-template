# AGENTS.md - virt-template

## Strict Rules

- Never modify generated files by hand - use `make generate`, `make manifests`, and `make vendor`
- Never remove or modify Apache 2.0 license headers (see `hack/boilerplate.go.txt`)
- Never bypass linting or skip `make all` before pushing
- Never commit vendor changes without running `make vendor`
- Never modify CRD type definitions without running `make generate` and `make manifests` afterward

## Project Overview

virt-template is a KubeVirt add-on that provides native VM templating within Kubernetes. Users define reusable VM blueprints as `VirtualMachineTemplate` custom resources with parameter placeholders (`${PARAM}`), then process them server-side or via CLI to create VirtualMachines. A companion `VirtualMachineTemplateRequest` CR creates templates from existing VMs (golden image workflow).

## Architecture

Kubebuilder v4 project with three binaries:

- **controller manager** (`cmd/main.go`) - reconciles VirtualMachineTemplate and VirtualMachineTemplateRequest CRs
- **API server** (`cmd/apiserver/main.go`) - serves `process` and `create` subresources on VirtualMachineTemplate
- **virttemplatectl** (`cmd/virttemplatectl/main.go`) - CLI tool / kubectl plugin for local and remote template processing

### Multi-module Go workspace

The project uses `go.work` with four modules:

| Module | Path | Purpose |
|--------|------|---------|
| `kubevirt.io/virt-template` | `.` | Main module (controllers, webhooks, apiserver, CLI) |
| `kubevirt.io/virt-template-api` | `./api` | Public API types (CRD structs) |
| `kubevirt.io/virt-template-client-go` | `./staging/src/kubevirt.io/virt-template-client-go` | Generated typed client |
| `kubevirt.io/virt-template-engine` | `./staging/src/kubevirt.io/virt-template-engine` | Template processing engine |

### Key directories

```
api/core/v1alpha1/       - CRD type definitions (VirtualMachineTemplate, VirtualMachineTemplateRequest)
api/core/subresourcesv1alpha1/ - Subresource types (ProcessOptions, CreateOptions)
internal/controller/     - Reconcilers
internal/webhook/        - Validation webhooks
internal/apiserver/      - REST storage and subresource handlers
internal/virttemplatectl/ - CLI commands (process, convert, create, templates)
staging/.../virt-template-engine/template/ - Parameter substitution, generation, visitor pattern
config/                  - Kubebuilder-based Kustomize overlays (default, openshift, virt-operator), CRDs, RBAC, webhooks
tests/                   - Functional/integration tests (Ginkgo)
hack/                    - Build scripts, code generation, linting
```

## Build and Development

### Prerequisites

- Go (check `go.mod` for the required version)
- Podman (preferred) or Docker
- Tools auto-download to `./bin/` on first use

### Building binaries

```
make build               - Build controller manager
make build-apiserver     - Build API server
make build-virttemplatectl - Build CLI for all platforms
```

### Key make targets

Run `make all` as the pre-commit validation step - it formats, vets, vendors, lints, regenerates manifests and code, and checks for uncommitted changes.

```
make all                 - fmt, vet, vendor, lint, manifests, generate, check-uncommitted
make test                - Unit tests with coverage
make functest            - Functional tests (requires cluster)
make lint                - golangci-lint + hack/lint.sh + license header check
make fmt                 - gofumpt formatting
make manifests           - Generate CRDs, RBAC, webhook configs via controller-gen
make generate            - Generate DeepCopy, OpenAPI, client code
make vendor              - Tidy all modules + go work vendor
make container-build     - Build multi-arch container images (restrict to single arch with IMG_PLATFORMS=linux/amd64)
```

### Cluster development

```
make cluster-up          - Start kubevirtci cluster with stable KubeVirt
make cluster-sync        - Build, push, deploy to cluster
make cluster-functest    - Run functional tests on cluster
make cluster-down        - Tear down cluster
```

Variants `kubevirt-up/sync/functest/down` use KubeVirt from git main instead.

## Testing

- **Unit tests**: standard Go test + Ginkgo/Gomega. Run with `make test`. Uses envtest (etcd + apiserver binaries).
- **Functional tests**: Ginkgo in `tests/` directory. Run with `make functest` or `make cluster-functest`. Always randomized (`-ginkgo.randomize-all`). Requires `KUBECONFIG` - skips if not set. Prefer `make cluster-sync cluster-functest` to build, deploy, and test in one step when a cluster is available.
- Controller tests are split into focused files under `internal/controller/` (e.g. `vmtr_finalizer_test.go`, `vmtr_datavolume_handling_test.go`).
- Engine tests live in `staging/src/kubevirt.io/virt-template-engine/template/`.

### Test patterns

- Each package has a `suite_test.go` with `BeforeSuite`/`AfterSuite` for envtest setup.
- Controller tests create random namespaces per test for isolation.
- Use `fake.NewClientBuilder().WithScheme(testScheme).WithStatusSubresource(...).Build()` for unit tests with fake clients.
- Helper builders in `vmtr_common_test.go`: `createRequest()`, `createSnapshot()`, `setSnapshotStatus()`, `createDataVolume()`, `expectCondition()`.
- Functional tests use `Eventually` with 5-minute timeouts for async operations.
- Prefer `DescribeTable` with `Entry` for parameterized test cases.
- Webhook tests start a real webhook server with TLS in `BeforeSuite`.

## Code Generation

Generated code must be committed. After changing API types or RBAC markers:

1. `make generate` - DeepCopy, OpenAPI schema, typed client
2. `make manifests` - CRDs, RBAC roles, webhook configs
3. `make vendor` - sync go.work and vendor directory

The `make all` target runs all of these plus formatting and linting.

## Linting

Run `make lint` to execute all linters:

- **golangci-lint** (version managed in `Makefile`) with `.golangci.yml` - line length 140, cyclomatic complexity 15, function length 100 lines / 50 statements
- **hack/lint.sh** - runs `yamllint` on `config/` and `shellcheck` on `hack/*.sh` (both must be installed on the host)
- **hack/license-header-check.sh** - Apache 2.0 header (see `hack/boilerplate.go.txt`)
- **gofumpt** - formatting (stricter than gofmt)

## Template Engine

Two parameter substitution syntaxes:

- `${PARAM}` - string substitution, supports multiple per field (e.g. `"${A}-${B}"`)
- `${{PARAM}}` - non-string substitution, replaces entire value, drops quotes, result parsed as JSON (numbers, booleans, objects)

Parameter generation uses `"expression"` generator with character classes: `\w`, `\d`, `\a`, `\A`, `[a-z]`, `[0-9]`, etc. Format: `[charset]{length}`.

Processing flow: generate parameter values - remove hardcoded namespace (unless parametrized) - substitute parameters - validate VM.

## Controller Design

### VirtualMachineTemplate controller

Minimal - marks templates as Ready.

### VirtualMachineTemplateRequest controller

Multi-step pipeline with requeue:

1. Create VirtualMachineSnapshot of source VM
2. Wait for snapshot readiness (requeue after 10s)
3. Clone snapshot volumes to DataVolumes
4. Wait for DataVolume readiness (requeue after 10s)
5. Expand VM spec (remove instance types/preferences)
6. Create VirtualMachineTemplate
7. Transfer DataVolume ownership from request to template
8. Delete snapshot

Key patterns:
- Finalizer `template.kubevirt.io/SnapshotCleanup` for cleanup on deletion
- `Progressing=True` + `Ready=False` means in-progress (will requeue). `Progressing=False` + `Ready=False` means permanent failure (stops).
- Objects tracked by `template.kubevirt.io/RequestUID` label on child resources
- Deterministic child object names via FNV-32a hash (`internal/apimachinery/naming.go`)
- VirtualMachineTemplateRequest spec is immutable (CEL rule: `self == oldSelf`)

### Cross-namespace authorization

ValidatingAdmissionPolicy with CEL checks three permissions when creating a VirtualMachineTemplateRequest:
- `virtualmachinetemplaterequests/source` create in source VM's namespace (skipped if same namespace)
- `datavolumes` create in target namespace
- `virtualmachinetemplates` create in target namespace

## API Server

Aggregated API server serving subresources only (no direct storage for the parent CRD):
- `POST /virtualmachinetemplates/{name}/process` - process template, return VM
- `POST /virtualmachinetemplates/{name}/create` - process template + create VM in cluster

Parent resource uses a dummy REST storage required by the k8s.io/apiserver framework. The APIResourceList is filtered to hide it.

## CLI Tool (virttemplatectl)

Also works as kubectl plugin (`kubectl virttemplate`) via Krew symlink detection.

Subcommands:
- `process` - process a template from file (`-f`) or cluster (`--name`), output YAML/JSON. `--create` to create VM via server subresource. `--local` forces local processing.
- `convert` - convert OpenShift Template to VirtualMachineTemplate
- `create` - create a VirtualMachineTemplateRequest from an existing VM (`--vm-name`, `--vm-namespace`)

## Deployment

Three Kustomize overlays in `config/`:
- `default` - Kubernetes with cert-manager (self-signed issuer)
- `openshift` - OpenShift with Service CA operator (different namespace: `openshift-cnv`, different DNS labels)
- `virt-operator` - certificates managed externally by virt-operator, ingress-only network policies

All overlays set namespace prefix `virt-template-` and deploy to their respective namespace.

## Conventions

- API group: `template.kubevirt.io`, version `v1alpha1`
- All source files carry Apache 2.0 license headers from `hack/boilerplate.go.txt`
- Commit messages: conventional commits with scope, e.g. `feat(config,admission): ...`, `fix(virttemplatectl,process): ...`
- Commits must be signed off (`git commit -s`)
- PRs and issues must follow the GitHub templates in `.github/` (`.github/PULL_REQUEST_TEMPLATE.md`, `.github/ISSUE_TEMPLATE.md`). Always read the template before creating a PR or issue and fill in all sections.
- Container images: `quay.io/kubevirt/virt-template-controller` and `virt-template-apiserver`
- Multi-arch: linux/amd64, linux/arm64, linux/s390x; CLI adds darwin and windows
- Generated client code lives in `staging/`; expansion interfaces (`*_expansion.go`) are hand-written
- Logging levels: `V(1)` for debug, `V(2)` for trace
