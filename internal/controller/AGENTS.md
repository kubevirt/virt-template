# AGENTS.md - internal/controller

Reconcilers for `VirtualMachineTemplate` and `VirtualMachineTemplateRequest`.

## VirtualMachineTemplate controller

Minimal - marks templates as `Ready`.

## VirtualMachineTemplateRequest controller

Multi-step pipeline with requeue:

1. Create `VirtualMachineSnapshot` of source VM
2. Wait for `VirtualMachineSnapshotContent` readiness (requeue after 10s)
3. Clone snapshot volumes to `DataVolume`s
4. Wait for `DataVolume` readiness (event-driven requeue; no fixed interval)
5. Expand VM spec (conditionally removes instance types/preferences - cluster-scoped, fully resolved matchers without `InferFromVolume` are retained)
6. Create `VirtualMachineTemplate`
7. Transfer `DataVolume` ownership from request to template
8. Delete snapshot

Key patterns - preserve these when touching the reconciler:

- Finalizer `template.kubevirt.io/SnapshotCleanup` for cleanup on deletion
- `Progressing=True` + `Ready=False` means in-progress (will requeue). `Progressing=False` + `Ready=False` means permanent failure (stops)
- Objects tracked by `template.kubevirt.io/RequestUID` label on child resources
- Deterministic child object names via FNV-32a hash (`internal/apimachinery/naming.go`)
- `VirtualMachineTemplateRequest` spec is immutable (CEL rule: `self == oldSelf`) - the reconciler can safely assume the spec remains constant throughout its lifecycle.

Cross-namespace authorization is enforced by a `ValidatingAdmissionPolicy` in `config/admission/` - see `config/AGENTS.md`.

## Test patterns

- Tests are split into focused files (e.g. `vmtr_finalizer_test.go`, `vmtr_datavolume_handling_test.go`)
- Each package has a `suite_test.go` with `BeforeSuite`/`AfterSuite` for envtest setup
- Tests create a random namespace per test for isolation
- Use `fake.NewClientBuilder().WithScheme(testScheme).WithStatusSubresource(...).Build()` for unit tests with fake clients, registering `WithStatusSubresource` for any type whose status is updated
- Shared helper builders live in `vmtr_common_test.go`: `createRequest()`, `createSnapshot()`, `setSnapshotStatus()`, `createDataVolume()`, `expectCondition()`
- Prefer `DescribeTable` with `Entry` for parameterized test cases
