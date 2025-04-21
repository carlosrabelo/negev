# `--group`

```text
--group <tag>
```

Executa contra switches cujas `tags` contêm `<tag>`.

## Quando usar

Andares, papéis (`access`, `core`) ou qualquer tag definida no YAML.

## Exemplos

```bash
negev --group access --config docs/examples/config.yaml
negev --group building-a --config docs/examples/config.yaml --write
```

## Interações

Mutuamente exclusivo com `--target` e `--all`. Conjunto vazio é erro (código `1`).
