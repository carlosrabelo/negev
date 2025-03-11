# Negev User Guide

Negev automates VLAN assignment on Cisco IOS access switches from MAC address prefixes in a YAML file.

## Connecting

Use Telnet or SSH. Telnet sends credentials in cleartext. SSH host-key verification is disabled — use only on trusted networks.

```bash
negev --target 192.168.1.10
negev --target 192.168.1.10 --write
```

## Configuration

See `demos/config.yaml`. Platforms: `ios`, `dmos`, and `auto`.
