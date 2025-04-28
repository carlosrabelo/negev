# `--jump-ask-pass`

```text
--jump-ask-pass
```

Prompt once on the terminal for the jump host SSH password (masked). Overrides YAML `jump_host.password` / `password_command` for the selected targets.

## Examples

```bash
negev --tag access --config docs/examples/config.yaml --jump ops@192.0.2.5 --jump-ask-pass
```

## Notes

Requires an active jump host (`--jump` or YAML `jump_host`); cannot be combined with `--jump none`. Supports both SSH `password` and `keyboard-interactive` authentication.
