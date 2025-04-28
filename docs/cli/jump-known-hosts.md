# `--jump-known-hosts`

```text
--jump-known-hosts <path>
```

Path to the OpenSSH `known_hosts` file used to verify the jump host's SSH host key. Overrides YAML `jump_host.known_hosts` (supports `~/`).

## Defaults

If omitted and `jump_host.known_hosts` is empty, Negev checks `~/.ssh/known_hosts`. Jump host connections fail if the file is missing or the host key does not match.

## Examples

```bash
negev --all --config docs/examples/config.yaml --jump ops@192.0.2.5 --jump-known-hosts ~/.ssh/known_hosts_bastion
```

## Notes

Requires an active jump host (`--jump` or YAML `jump_host`); cannot be combined with `--jump none`.
