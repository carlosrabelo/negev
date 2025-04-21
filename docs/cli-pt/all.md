# `--all`

```text
--all
```

Executa contra todos os switches do YAML carregado.

## Quando usar

Rodadas em toda a frota (sandbox ou write).

## Exemplos

```bash
negev --all --config docs/examples/config.yaml
negev --all --config docs/examples/config.yaml --concurrency 4 --write
```

## Interações

Mutuamente exclusivo com `--target` e `--group`.

## Notas

A saída é bufferizada por alvo; uma tabela-resumo aparece ao final. Código `1` se qualquer alvo falhar.
