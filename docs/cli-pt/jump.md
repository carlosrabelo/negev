# `--jump`

```text
--jump <[user@]host[:port]|none>
```

Encaminha conexões Telnet e SSH aos switches através de um único jump host SSH (bastion) usando encaminhamento `direct-tcpip`. Sobrescreve o campo `jump_host` do YAML para os alvos selecionados.

## Quando usar

Quando os switches estão em uma rede de gerência isolada acessível apenas por um bastion (por exemplo via VPN).

## Exemplos

```bash
negev --tag access --config docs/examples/config.yaml --jump ops@192.0.2.5
negev --target 192.0.2.10 --config docs/examples/config.yaml --jump ops@192.0.2.5:2222 --write
negev --target 192.0.2.10 --config docs/examples/config.yaml --jump none
```

## Notas

- A verificação de chave do jump host é obrigatória contra `~/.ssh/known_hosts` (ajuste o caminho com [`--jump-known-hosts`](jump-known-hosts.md)).
- A autenticação tenta `--jump-key` / `jump_host.key_file`, `ssh-agent`, chaves padrão `~/.ssh/id_{ed25519,ecdsa,rsa}` e senha / `--jump-ask-pass`.
- Use `none` (ou `off`, `direct`, `false`) para desativar um `jump_host` do YAML em uma execução pontual.
- Uma única conexão SSH com o jump host é multiplexada entre os workers concorrentes de `--concurrency`.
