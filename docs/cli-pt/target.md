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

Mutuamente exclusivo com `--tag`. É obrigatório um de `--target` ou `--tag`.

## Notas

IP/`name` desconhecido falha com código `1`.
