# `--write`

```text
--write
```

Apply changes on the switch. Without this flag, Negev runs in sandbox mode and only prints `SIMULATE:` lines.

## When to use

After a clean sandbox preview.

## Examples

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml
negev --target 192.0.2.10 --config docs/examples/config.yaml --write
```

## Notes

- Sandbox exit `2` means simulated changes with no failures.
- `--write` triggers a pre-change running-config backup (unless backup fails and aborts).
