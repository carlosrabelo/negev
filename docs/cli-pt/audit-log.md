# `--audit-log`

```text
--audit-log <path>
```

Anexa registros JSONL para cada mudança de VLAN de porta (sucesso ou falha). Sobrescreve o YAML `audit_log`. Modo do arquivo `0644`, sync por entrada.

## Exemplos

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml --write --audit-log ./audit/negev.jsonl
```

## Campos

`timestamp`, `user`, `target`, `switch_name`, `port`, `mac`, `old_vlan`, `new_vlan`, `action` (`SET_PORT_VLAN`), `sandbox`, `status`.
