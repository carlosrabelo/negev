# `--tag`

```text
--tag <tag>
```

Executa contra switches cujo campo `tag` no YAML (string ou lista) contém `<tag>`.

## Quando usar

Agrupar switches por filial, unidade, andar ou função (`access`, `core`) no YAML sem rodar acidentalmente em todo o inventário.

## Exemplos

```bash
negev --tag access --config docs/examples/config.yaml
negev --tag building-a --config docs/examples/config.yaml --write
```

## Interações

Mutuamente exclusivo com `--target`. Conjunto vazio é erro (código `1`). O campo legado `tags:` no YAML também é aceito e mesclado.
