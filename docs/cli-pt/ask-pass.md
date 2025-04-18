# `--ask-pass`

```text
--ask-pass
```

Pergunta uma vez no terminal a senha do switch e a senha enable opcional (mascaradas). Sobrescreve YAML/`password_command`/env para o(s) alvo(s).

## Exemplos

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml --ask-pass
negev --target access-a1 --config docs/examples/config.yaml --ask-pass --verbose 1
```

## Notas

Prefira expansão `${ENV}` ou `password_command` em automação; use `--ask-pass` em sessões interativas.
