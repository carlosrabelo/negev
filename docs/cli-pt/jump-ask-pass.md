# `--jump-ask-pass`

```text
--jump-ask-pass
```

Pergunta uma vez no terminal a senha SSH do jump host (mascarada). Sobrescreve `jump_host.password` / `password_command` no YAML para os alvos selecionados.

## Exemplos

```bash
negev --all --config docs/examples/config.yaml --jump ops@192.0.2.5 --jump-ask-pass
```

## Notas

Exige um jump host ativo (`--jump` ou `jump_host` no YAML); não pode ser combinado com `--jump none`. Suporta autenticação SSH `password` e `keyboard-interactive`.
