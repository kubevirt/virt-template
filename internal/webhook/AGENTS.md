# AGENTS.md - internal/webhook

Validation webhooks for `VirtualMachineTemplate` and `VirtualMachineTemplateRequest`.

## Rules

- Validate parameter placeholder syntax (`${PARAM}` / `${{PARAM}}`) on `VirtualMachineTemplate` writes - this is the sole concern of the webhook validators
- `VirtualMachineTemplateRequest` spec immutability is enforced by a CEL rule on the API type (see `api/core/AGENTS.md`), not by a webhook in this package

Note: cross-namespace authorization for `VirtualMachineTemplateRequest` is enforced by a `ValidatingAdmissionPolicy`, not by code in this package - see `config/AGENTS.md`.

## Test patterns

- Webhook tests start a real webhook server with TLS in `BeforeSuite`
