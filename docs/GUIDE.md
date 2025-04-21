# Negev User Guide

Negev is a CLI tool for automating VLAN assignments on network switches based on MAC address prefixes.

## Installation

```bash
make install
```

This installs the binary to `~/.local/bin/negev` (Linux) or `/usr/local/bin/negev`.

## Configuration

Create a `config.yaml` file. Negev searches for it in:

- `./config.yaml`
- `~/.config/negev/config.yaml` (Linux)
- `/etc/negev/config.yaml` (Linux)
- `%APPDATA%\negev\config.yaml` (Windows)
- `%ProgramData%\negev\config.yaml` (Windows)

### Global Settings

```yaml
platform: auto
transport: telnet
username: admin
password: cisco123
enable_password: cisco123
default_vlan: "1"
no_data_vlan: "999"
mac_to_vlan:
  "aabbcc": "10"
```

### Per-Switch Overrides

```yaml
switches:
  - target: 192.168.1.10
    platform: ios
    mac_to_vlan:
      "001122": "30"
```

## Usage

```bash
negev --target 192.168.1.10
```

### Flags

| Flag | Description |
|---|---|
| `--target <ip>` | Switch IP address (required) |
| `--config <path>` | Path to YAML config file |
| `--write` | Apply changes (sandbox by default) |
| `--verbose <0-3>` | 0=none, 1=debug, 2=raw, 3=both |
| `--create-vlans` | Create/delete VLANs to match allowed list |
| `--version` | Show version |

## How It Works

1. Connects to the switch via Telnet or SSH
2. Detects platform (Cisco IOS) automatically or uses configured driver
3. Reads the MAC address table and active ports
4. Maps MAC prefixes to VLANs using the configuration
5. Configures access VLANs on switch ports
6. Displays simulated changes in sandbox mode (use `--write` to apply)

## Sandbox Mode

By default, Negev runs in sandbox mode showing what would be changed without applying anything. Use `--write` to apply changes.

```bash
negev --target 192.168.1.10          # sandbox (simulate)
negev --target 192.168.1.10 --write  # apply changes
```

## MAC Address Format

MAC addresses are normalized to 12 lowercase hex characters:

- `aa:bb:cc:dd:ee:ff` → `aabbccddeeff`
- `aabb.ccdd.eeff` → `aabbccddeeff`

The VLAN mapping uses the first 6 characters (prefix) as the key.

## VLAN Protection

- VLANs 1000–4094 are automatically protected
- Additional VLANs can be listed in `protected_vlans`
- Protected VLANs are never deleted, even with `--create-vlans`
