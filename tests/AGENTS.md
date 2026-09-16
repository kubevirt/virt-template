# AGENTS.md - tests

Functional/integration tests (Ginkgo), run against a real cluster.

## Rules

- Run with `make functest` or `make cluster-functest`; always randomized (`-ginkgo.randomize-all`)
- Requires `KUBECONFIG` - tests must skip cleanly when it's unset, not fail
- Use `Eventually` with appropriately long timeouts (5 minutes is typical) for async operations (snapshot readiness, DataVolume readiness, VM creation, etc.)
- Prefer `DescribeTable` with `Entry` for parameterized test cases
