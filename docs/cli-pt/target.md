# `--target`

```text
--target <ip|name>[,...]
```

Seleciona um ou mais switches por IP e/ou `name` configurado, separados por vírgula.

## Quando usar

Execuções em um switch ou em alvos explícitos.

## Exemplos

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml
negev --target access-a1,192.0.2.20 --config docs/examples/config.yaml
```

## Interações

Mutuamente exclusivo com `--all` e `--group`. É obrigatório um de `--target`, `--all` ou `--group`.

## Notas

IP/`name` desconhecido falha com código `1`.
