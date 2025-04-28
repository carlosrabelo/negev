# `--jump-key`

```text
--jump-key <path>
```

Path to the SSH private key used to authenticate against the jump host. Overrides YAML `jump_host.key_file` (supports `~/`).

## Examples

```bash
negev --tag access --config docs/examples/config.yaml --jump ops@192.0.2.5 --jump-key ~/.ssh/id_bastion
```

## Notes

Requires an active jump host (`--jump` or YAML `jump_host`); cannot be combined with `--jump none`. If the key is encrypted and not loaded in `ssh-agent`, configure `jump_host.key_passphrase` or `key_passphrase_command`.
