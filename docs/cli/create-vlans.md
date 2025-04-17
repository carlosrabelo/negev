# `--create-vlans`

```text
--create-vlans
```

Synchronize the switch VLAN database with `allowed_vlans`: create missing VLANs; delete non-allowed, non-protected VLANs. VLAN `1` and range 1000–4094 are always protected; `default_vlan`, `no_data_vlan`, `protected_vlans`, L3 SVIs, and VLANs still on active ports are also protected.

## Examples

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml --create-vlans
negev --target 192.0.2.10 --config docs/examples/config.yaml --create-vlans --write
```

## Notes

Requires a non-empty `allowed_vlans` list. Preview in sandbox first.
