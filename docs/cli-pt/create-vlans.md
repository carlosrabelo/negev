# `--create-vlans`

```text
--create-vlans
```

Sincroniza o banco de VLANs do switch com `allowed_vlans`: cria faltantes; remove não-allowed não-protegidas. VLAN `1` e a faixa 1000–4094 são sempre protegidas; `default_vlan`, `no_data_vlan`, `protected_vlans`, SVIs L3 e VLANs ainda em portas ativas também.

## Exemplos

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml --create-vlans
negev --target 192.0.2.10 --config docs/examples/config.yaml --create-vlans --write
```

## Notas

Exige `allowed_vlans` não vazio. Pré-visualize no sandbox primeiro.
