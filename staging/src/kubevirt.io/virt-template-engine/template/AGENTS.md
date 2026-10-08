# AGENTS.md - virt-template-engine/template

Parameter substitution, generation, and visitor pattern (module `kubevirt.io/virt-template-engine`).

## Substitution syntaxes

Two parameter substitution syntaxes - verify correct handling of both when touching this package:

- `${PARAM}` - string substitution, supports multiple per field (e.g. `"${A}-${B}"`)
- `${{PARAM}}` - non-string substitution, replaces the entire value, drops quotes, result parsed as JSON (numbers, booleans, objects)

## Parameter generation

Uses an `"expression"` generator with character classes: `\w`, `\d`, `\a`, `\A`, `[a-z]`, `[0-9]`, etc. Format: `[charset]{length}`.

## Processing order

The generate -> strip-namespace -> substitute -> validate order must be preserved:

1. Generate parameter values
2. Remove hardcoded namespace (unless parametrized)
3. Substitute parameters
4. Validate the resulting VM

## Test patterns

Engine tests live alongside the code in this package.
