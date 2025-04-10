# `--target`

```text
--target <ip|name>[,...]
```

Select one or more switches by IP and/or configured `name`, comma-separated.

## When to use

Single-switch or explicit multi-switch runs when you know the targets.

## Examples

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml
negev --target access-a1,192.0.2.20 --config docs/examples/config.yaml
```

## Interactions

Mutually exclusive with `--all` and `--group`. At least one of `--target`, `--all`, or `--group` is required.

## Notes

Unknown IP/name fails with exit code `1`.
