# `--create-vlans`

```text
--create-vlans
```

Sincroniza o banco de VLANs do switch com `allowed_vlans`: cria faltantes; remove não-allowed não-protegidas. A faixa 1000–4094 é sempre protegida; `protected_vlans` adiciona mais.

## Exemplos

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml --create-vlans
negev --target 192.0.2.10 --config docs/examples/config.yaml --create-vlans --write
```

## Notas

Exige `allowed_vlans` não vazio. Pré-visualize no sandbox primeiro.
