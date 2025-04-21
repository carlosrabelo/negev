# `--write`

```text
--write
```

Aplica mudanças no switch. Sem esta flag, o Negev roda em sandbox e só imprime linhas `SIMULATE:`.

## Quando usar

Depois de um preview limpo no sandbox.

## Exemplos

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml
negev --target 192.0.2.10 --config docs/examples/config.yaml --write
```

## Notas

- Saída `2` no sandbox indica mudanças simuladas sem falhas.
- `--write` dispara backup do running-config antes das mutações (falha de backup aborta).
