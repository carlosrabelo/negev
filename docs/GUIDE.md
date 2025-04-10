# Negev User Guide

Negev is a CLI tool for automating VLAN assignments on network access switches based on MAC address prefixes. It connects to network switches via Telnet or SSH, reads the MAC address table, maps connected devices to their target VLANs, and configures the switch ports accordingly.

## Installation

To compile and install the binary from source, run:

```bash
make install
```

This installs the binary to `~/.local/bin/negev` (Linux) or `/usr/local/bin/negev`.

## Configuration

Negev uses a YAML configuration file. By default, it searches for `config.yaml` in the current directory, but it also falls back to the following system paths:

- `./config.yaml`
- `~/.config/negev/config.yaml` (Linux)
- `/etc/negev/config.yaml` (Linux)
- `%APPDATA%\negev\config.yaml` (Windows)
- `%ProgramData%\negev\config.yaml` (Windows)

### Configuration Schema

The configuration structure supports global default settings that can be overridden on a per-switch basis.

#### Global Settings

These settings apply to all switches unless overridden:

```yaml
# Supported: auto, ios, dmos. "auto" runs 'show version' to detect the platform.
platform: auto

# Supported: telnet, ssh
transport: telnet

# Global authentication credentials
username: admin
password: cisco123
enable_password: cisco123

# Default VLAN for active ports with unrecognized MAC addresses
default_vlan: "1"

# Optional quarantine VLAN for link-up access ports with no learned MAC.
# Omit to leave those ports unchanged. Aging of quiet devices can isolate them.
no_data_vlan: "999"

# Global list of allowed VLANs that Negev is allowed to create or modify
allowed_vlans:
  - "10"
  - "20"
  - "30"

# Global list of protected VLANs that should never be deleted
protected_vlans:
  - "100"

# Global MAC addresses to ignore (exact matches, normalized)
exclude_macs:
  - "00:11:22:33:44:55"

# Global mapping of MAC prefixes (first 6 hex digits) to VLAN IDs
mac_to_vlan:
  "aabbcc": "10"
  "001122": "20"
```

#### Per-Switch Settings and Overrides

You can define individual switches in the `switches` block. Each switch can override credentials, transport, VLANs, and MAC-to-VLAN mappings:

```yaml
switches:
  - target: 192.168.1.10
    platform: ios
    transport: telnet
    mac_to_vlan:
      "001122": "30" # Override global mapping for prefix 001122

  - target: 192.168.1.20
    platform: dmos
    transport: ssh
    username: operator
    password: secure_password
    # Disable inheritance of a global MAC prefix mapping by setting it to "0", "00" or ""
    mac_to_vlan:
      "aabbcc": "0" 
    exclude_ports:
      - "ethernet 1/1"
      - "ethernet 1/2"
```

### Configuration Merging Rules

1. **MacToVlan Map**: Global prefix mappings are merged with switch-specific mappings. Switch mappings override global ones for the same prefix. If a switch mapping sets a prefix's VLAN to `"0"`, `"00"`, or `""`, that prefix mapping is removed entirely for that switch.
2. **ExcludeMacs List**: Global and switch-specific excluded MACs are merged, normalized, and deduplicated.
3. **ExcludePorts List**: Defined only at the switch level. Ports in this list are completely ignored during VLAN assignment.
4. **AllowedVlans & ProtectedVlans**: Switch-specific lists are merged with the global lists and deduplicated.

---

## Usage

```bash
negev --target <switch_ip> [flags]
```

### Flags

| Flag | Description |
|---|---|
| `--target <ip>` | Switch IP address to connect to (required, must exist in configuration) |
| `--config <path>` | Path to the YAML configuration file |
| `--write` | Apply changes to the switch (sandbox/dry-run mode is active by default) |
| `--verbose <0-3>` | Output verbosity: `0` = none, `1` = debug logs, `2` = raw switch communication, `3` = both |
| `--create-vlans` | Automatically create missing allowed VLANs and delete unauthorized ones (needs `--write` to apply) |
| `--version` | Display version and build time |

---

## How It Works

1. **Connection and Driver Resolution**: Negev connects to the switch using the configured transport (Telnet or SSH). If the platform is set to `auto`, it executes a `show version` command and detects whether it is a Cisco IOS or Datacom DmOS switch.
2. **Security & Client Caching**: 
   - **Telnet**: Transmits credentials in cleartext (logs a warning).
   - **SSH**: Uses VT100 PTY emulation with suppressed echo. Disables host-key validation (logs a warning).
   - **Caching**: Switches reuse cached network clients if they share the same credentials, transport, and target.
3. **Information Gathering**: Negev queries the switch for active VLANs, trunk ports, active interface statuses, and the dynamic MAC address table.
   - **Cisco IOS**: Uses `show vlan brief`, `show interfaces trunk`, `show interfaces status`, and `show mac address-table dynamic`.
   - **Datacom DmOS**: Uses `show vlan table`, `show interfaces switchport` (cached per-run to avoid duplicate calls), `show interfaces status`, and `show mac-address-table`.
4. **Trunk Port Exclusion**: Interfaces detected as trunk ports are automatically skipped to prevent network disruption.
5. **VLAN Assignment Logic**:
   - For each access port, Negev inspects the connected MAC address.
   - If multiple MACs are detected on the same port, a safety warning is logged and the port is skipped.
   - The MAC address is normalized (removing `:` and `.`, lowercasing) and its 6-character prefix is checked against `mac_to_vlan`.
   - If a matching prefix is found, the target VLAN is assigned.
   - If no prefix matches, the port is assigned to `default_vlan`.
   - If no MAC address is active on the port and `no_data_vlan` is set, the port is assigned to that quarantine VLAN. Only link-up access ports are considered; `notconnect` ports are left unchanged. Omit `no_data_vlan` to leave empty ports as they are. A quiet device whose MAC has aged out of the table can be moved to this VLAN and isolated.
   - If the target VLAN does not exist on the switch, the assignment is skipped with an error.
6. **VLAN Synchronization (`--create-vlans`)**:
   - Compares the active switch VLANs with the list of `allowed_vlans`.
   - Creates any VLAN defined in `allowed_vlans` that is missing on the switch.
   - Deletes any VLAN present on the switch that is *not* in `allowed_vlans` and is *not* protected.
7. **Execution or Simulation**: If `--write` is omitted, Negev displays the exact commands it would send. If `--write` is specified, commands are executed, and the configuration is saved (`write memory` on IOS, `copy running-config startup-config` on DmOS).

---

## Sandbox Mode

By default, Negev runs in a safe sandbox mode, allowing you to preview switch changes before applying them:

```bash
# Preview changes on Cisco IOS
negev --target 192.168.1.10

# Apply changes on Datacom DmOS via SSH
negev --target 192.168.1.20 --write
```

---

## MAC Address Normalization

MAC addresses are normalized by stripping separators (`:`, `.`) and converting them to lowercase:

- `00:11:22:AA:BB:CC` → `001122aabbcc`
- `0011.22aa.bbcc` → `001122aabbcc`

The lookup key in the `mac_to_vlan` mapping is the first 6 characters of the normalized MAC (e.g., `001122`).

---

## VLAN Protection

VLANs are protected from deletion (when running with `--create-vlans`) under two conditions:
- **Auto-protection**: VLANs in the range `1000` to `4094` are automatically protected.
- **Explicit protection**: VLANs listed in `protected_vlans` in the configuration file.

Protected VLANs are never deleted by Negev.

---

## Practical Scenarios

Walkthroughs formerly kept under a standalone `demos/` directory now live under `docs/`.

A complete sample configuration (documentation IPs only) is available at [`docs/examples/config.yaml`](examples/config.yaml).

### Scenario 1: Initial access-switch deployment

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml
# Review SIMULATE output, then:
negev --target 192.0.2.10 --config docs/examples/config.yaml --write
```

### Scenario 2: Synchronize the VLAN database

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml --create-vlans
negev --target 192.0.2.10 --config docs/examples/config.yaml --create-vlans --write
```

### Scenario 3: Segment unmanaged access ports

Map department OUI/MAC prefixes in `mac_to_vlan`, exclude uplink ports with `exclude_ports`, preview in sandbox, then apply with `--write`.

### Scenario 4: Troubleshoot prompts and pagers

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml --verbose 1   # debug
negev --target 192.0.2.10 --config docs/examples/config.yaml --verbose 2   # raw I/O
negev --target 192.0.2.10 --config docs/examples/config.yaml --verbose 3   # both
```
