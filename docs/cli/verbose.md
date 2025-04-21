# `--verbose`

```text
--verbose <0-3>
```

Verbosity level (default `0`):

| Level | Meaning |
|-------|---------|
| `0` | Quiet (errors and normal summaries only) |
| `1` | Debug |
| `2` | Raw switch I/O |
| `3` | Debug + raw I/O |

## Examples

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml --verbose 1
negev --target 192.0.2.10 --config docs/examples/config.yaml --verbose 2
negev --target 192.0.2.10 --config docs/examples/config.yaml --verbose 3
```

## When to use

Diagnose auth prompts, pagers, and parser issues.
