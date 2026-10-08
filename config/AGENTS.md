# AGENTS.md - config

Kubebuilder-based Kustomize manifests for deploying virt-template.

## Rules

- To change generated manifests here (CRDs, RBAC, webhook configs), update the source markers in `api/` type definitions or `+kubebuilder:rbac` markers in the code and run `make manifests` to regenerate
- Always keep YAML in this directory lint-clean (`hack/lint.sh` runs `yamllint` against it)

## Cross-namespace authorization

`config/admission/` defines a `ValidatingAdmissionPolicy` with CEL checks for three permissions when creating a `VirtualMachineTemplateRequest`:

- `virtualmachinetemplaterequests/source` create in the source VM's namespace (skipped if same namespace)
- `datavolumes` create in the target namespace
- `virtualmachinetemplates` create in the target namespace

## Overlays

- `default` - Kubernetes with cert-manager (self-signed issuer)
- `openshift` - OpenShift with Service CA operator (different namespace: `openshift-cnv`, different DNS labels)
- `virt-operator` - certificates managed externally by virt-operator, ingress-only network policies

All overlays set namespace prefix `virt-template-` and deploy to their respective namespace.
