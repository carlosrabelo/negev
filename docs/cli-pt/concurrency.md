# `--concurrency`

```text
--concurrency <n>
```

Máximo de execuções concorrentes (tamanho do pool). Padrão: `1`. Deve ser `>= 1`.

## Quando usar

Ajustar carga na rede ou bastion com `--tag` ou vários `--target`.

## Exemplos

```bash
negev --tag access --config docs/examples/config.yaml --concurrency 2
negev --tag building-a --config docs/examples/config.yaml --concurrency 8
```

## Notas

Cada worker bufferiza stdout/stderr até o alvo terminar e libera com prefixo `[label]`.
