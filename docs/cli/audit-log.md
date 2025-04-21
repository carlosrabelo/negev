# `--audit-log`

```text
--audit-log <path>
```

Append JSONL audit records for each port VLAN change (success or failure). Overrides YAML `audit_log`. File mode `0644`, sync per entry.

## Examples

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml --write --audit-log ./audit/negev.jsonl
```

## Fields

`timestamp`, `user`, `target`, `switch_name`, `port`, `mac`, `old_vlan`, `new_vlan`, `action` (`SET_PORT_VLAN`), `sandbox`, `status`.
