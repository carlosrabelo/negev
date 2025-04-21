# `--create-vlans`

```text
--create-vlans
```

Synchronize the switch VLAN database with `allowed_vlans`: create missing VLANs; delete non-allowed, non-protected VLANs. VLAN range 1000–4094 is always protected; `protected_vlans` adds more.

## Examples

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml --create-vlans
negev --target 192.0.2.10 --config docs/examples/config.yaml --create-vlans --write
```

## Notes

Requires a non-empty `allowed_vlans` list. Preview in sandbox first.
