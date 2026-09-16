# AGENTS.md - internal/apiserver

Aggregated API server REST storage and subresource handlers, serving subresources only (no direct storage for the parent CRD):

- `POST /virtualmachinetemplates/{name}/process` - process template, return VM
- `POST /virtualmachinetemplates/{name}/create` - process template + create VM in cluster

## Rules

- The parent resource uses a dummy REST storage required by the `k8s.io/apiserver` framework, and the `APIResourceList` is filtered to hide it - don't break either behavior
- RBAC is enforced via `SubjectAccessReview` delegation to the main API server (`DelegatingAuthorizationOptions`) - ensure all authorization checks rely on this mechanism.
- Handlers must surface upstream errors (e.g. from the processing engine or the cluster client) as proper Kubernetes API status errors (`apierrors.New*`) rather than opaque 500s
