# AGENTS.md - virt-template

## Strict Rules

- Never modify generated files by hand - use `make generate`, `make manifests`, and `make vendor`
- Never remove or modify Apache 2.0 license headers (see `hack/boilerplate.go.txt`)
- Never bypass linting or skip `make all` before pushing
- Never commit vendor changes without running `make vendor`
- Never modify CRD type definitions without running `make generate` and `make manifests` afterward

Several directories also carry their own nested `AGENTS.md` with directory-specific conventions (see "Key directories" below) - read the closest one for the file(s) you're touching, in addition to this file.

## Project Overview

virt-template is a KubeVirt add-on that provides native VM templating within Kubernetes. Users define reusable VM blueprints as `VirtualMachineTemplate` custom resources with parameter placeholders (`${PARAM}`), then process them server-side or via CLI to create VirtualMachines. A companion `VirtualMachineTemplateRequest` CR creates templates from existing VMs (golden image workflow).

## Architecture

Kubebuilder v4 project with two binaries:

- **controller manager** (`cmd/main.go`) - reconciles VirtualMachineTemplate and VirtualMachineTemplateRequest CRs
- **API server** (`cmd/apiserver/main.go`) - serves `process` and `create` subresources on VirtualMachineTemplate

### Multi-module Go workspace

The project uses `go.work` with four modules:

| Module | Path | Purpose |
|--------|------|---------|
| `kubevirt.io/virt-template` | `.` | Main module (controllers, webhooks, apiserver) |
| `kubevirt.io/virt-template-api` | `./api` | Public API types (CRD structs) |
| `kubevirt.io/virt-template-client-go` | `./staging/src/kubevirt.io/virt-template-client-go` | Generated typed client |
| `kubevirt.io/virt-template-engine` | `./staging/src/kubevirt.io/virt-template-engine` | Template processing engine |

### Key directories

```
api/core/                - CRD type definitions & subresource types - see api/core/AGENTS.md
internal/controller/     - Reconcilers - see internal/controller/AGENTS.md
internal/webhook/        - Validation webhooks - see internal/webhook/AGENTS.md
internal/apiserver/      - REST storage and subresource handlers - see internal/apiserver/AGENTS.md
staging/.../virt-template-engine/template/ - Parameter substitution/generation - see AGENTS.md there
config/                  - Kustomize overlays, CRDs, RBAC, webhooks, admission policy - see config/AGENTS.md
tests/                   - Functional/integration tests (Ginkgo) - see tests/AGENTS.md
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

- **Unit tests**: standard Go test + Ginkgo/Gomega. Run with `make test`. Uses envtest (etcd + apiserver binaries). Each package has a `suite_test.go` with `BeforeSuite`/`AfterSuite` for envtest setup.
- **Functional tests**: Ginkgo in `tests/` directory. Run with `make functest` or `make cluster-functest`. Prefer `make cluster-sync cluster-functest` to build, deploy, and test in one step when a cluster is available. See `tests/AGENTS.md`.
- Prefer `DescribeTable` with `Entry` for parameterized test cases, across both unit and functional tests.
- Package-specific test patterns (namespace isolation, fake client setup, helper builders, webhook TLS server, etc.) live in that package's nested `AGENTS.md` (e.g. `internal/controller/AGENTS.md`, `internal/webhook/AGENTS.md`).

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

Parameter substitution and generation for processing templates into VMs. Two substitution syntaxes (`${PARAM}` string substitution, `${{PARAM}}` non-string substitution) and a strict generate -> strip-namespace -> substitute -> validate processing order. See `staging/src/kubevirt.io/virt-template-engine/template/AGENTS.md` for details.

## Controller Design

`VirtualMachineTemplate` controller is minimal (marks templates `Ready`). `VirtualMachineTemplateRequest` controller runs a multi-step snapshot -> clone -> expand -> create pipeline with requeue, tracked via a finalizer, conditions, and deterministic child-object naming. See `internal/controller/AGENTS.md` for the full pipeline and invariants to preserve.

## Validation Webhooks

Two separate enforcement points:

- **`VirtualMachineTemplateRequest` spec immutability** is enforced by a CEL `self == oldSelf` rule declared on the API type, not by a webhook. See `api/core/AGENTS.md`.
- **`VirtualMachineTemplate` parameter placeholder validity** is enforced by a validating webhook. See `internal/webhook/AGENTS.md`.

## API Server

Aggregated API server serving subresources only (no direct storage for the parent CRD):
- `POST /virtualmachinetemplates/{name}/process` - process template, return VM
- `POST /virtualmachinetemplates/{name}/create` - process template + create VM in cluster

See `internal/apiserver/AGENTS.md` for the dummy REST storage/APIResourceList filtering, RBAC, and error-handling conventions.

## Deployment

Three Kustomize overlays in `config/` (default, openshift, virt-operator), each setting namespace prefix `virt-template-`. See `config/AGENTS.md` for overlay details and manifest hand-editing rules.

## Conventions

- API group: `template.kubevirt.io`, version `v1alpha1`
- All source files carry Apache 2.0 license headers from `hack/boilerplate.go.txt`
- Commit messages: conventional commits with scope, e.g. `feat(config,admission): ...`
- Commits must be signed off (`git commit -s`)
- PRs and issues must follow the GitHub templates in `.github/` (`.github/PULL_REQUEST_TEMPLATE.md`, `.github/ISSUE_TEMPLATE.md`). Always read the template before creating a PR or issue and fill in all sections.
- Container images: `quay.io/kubevirt/virt-template-controller` and `virt-template-apiserver`
- Multi-arch: linux/amd64, linux/arm64, linux/s390x
- Generated client code lives in `staging/`; expansion interfaces (`*_expansion.go`) are hand-written
- Logging levels: `V(1)` for debug, `V(2)` for trace
