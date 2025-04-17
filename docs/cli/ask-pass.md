# `--ask-pass`

```text
--ask-pass
```

Prompt once on the terminal for switch password and optional enable password (masked). Overrides YAML/`password_command`/env for the selected target(s).

## Examples

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml --ask-pass
negev --target access-a1 --config docs/examples/config.yaml --ask-pass --verbose 1
```

## Notes

Prefer `${ENV}` expansion or `password_command` for automation; use `--ask-pass` for interactive one-offs.
