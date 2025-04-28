# `--jump-key`

```text
--jump-key <path>
```

Caminho da chave privada SSH usada para autenticar no jump host. Sobrescreve `jump_host.key_file` no YAML (suporta `~/`).

## Exemplos

```bash
negev --tag access --config docs/examples/config.yaml --jump ops@192.0.2.5 --jump-key ~/.ssh/id_bastion
```

## Notas

Exige um jump host ativo (`--jump` ou `jump_host` no YAML); não pode ser combinado com `--jump none`. Se a chave tiver passphrase e não estiver no `ssh-agent`, configure `jump_host.key_passphrase` ou `key_passphrase_command`.
