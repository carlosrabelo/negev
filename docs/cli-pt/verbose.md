# `--verbose`

```text
--verbose <0-3>
```

Nível de verbosidade (padrão `0`):

| Nível | Significado |
|-------|-------------|
| `0` | Quieto (erros e resumos) |
| `1` | Debug |
| `2` | I/O bruto do switch |
| `3` | Debug + I/O bruto |

## Exemplos

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml --verbose 1
negev --target 192.0.2.10 --config docs/examples/config.yaml --verbose 2
negev --target 192.0.2.10 --config docs/examples/config.yaml --verbose 3
```

## Quando usar

Diagnosticar prompts de auth, pagers e parsers.
