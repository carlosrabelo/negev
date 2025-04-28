# `--jump-known-hosts`

```text
--jump-known-hosts <path>
```

Caminho do arquivo OpenSSH `known_hosts` usado para validar a chave pública do jump host. Sobrescreve `jump_host.known_hosts` no YAML (suporta `~/`).

## Padrões

Se omitido e `jump_host.known_hosts` estiver vazio, o Negev consulta `~/.ssh/known_hosts`. Conexões via jump host falham se o arquivo não existir ou a chave do servidor divergir.

## Exemplos

```bash
negev --tag access --config docs/examples/config.yaml --jump ops@192.0.2.5 --jump-known-hosts ~/.ssh/known_hosts_bastion
```

## Notas

Exige um jump host ativo (`--jump` ou `jump_host` no YAML); não pode ser combinado com `--jump none`.
