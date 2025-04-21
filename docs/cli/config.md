# `--config`

```text
--config <path>
```

Path to the YAML configuration file.

## When to use

Point at a non-default file (recommended in docs: `docs/examples/config.yaml`).

## Examples

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml
```

## Defaults

If omitted, Negev searches for `config.yaml` in:

1. `./config.yaml`
2. `~/.config/negev/config.yaml` (Linux/macOS user config dir)
3. `/etc/negev/config.yaml` (Linux)
4. `%APPDATA%\negev\config.yaml` / `%ProgramData%\negev\config.yaml` (Windows)

After resolving the path (via search or `--config`), stderr always prints
`Using config: <path>`.

## Notes

Full schema and merge rules: [GUIDE.md](../GUIDE.md). Sample file:
[`docs/examples/config.yaml`](../examples/config.yaml).
