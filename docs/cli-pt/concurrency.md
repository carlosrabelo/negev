# `--concurrency`

```text
--concurrency <n>
```

Máximo de execuções concorrentes (tamanho do pool). Padrão: `4`. Deve ser `>= 1`.

## Quando usar

Ajustar carga na rede ou bastion com `--all` / `--group` / vários `--target`.

## Exemplos

```bash
negev --all --config docs/examples/config.yaml --concurrency 2
negev --group access --config docs/examples/config.yaml --concurrency 8
```

## Notas

Cada worker bufferiza stdout/stderr até o alvo terminar e libera com prefixo `[label]`.
