# Negev

[![Go Report Card](https://goreportcard.com/badge/github.com/carlosrabelo/negev)](https://goreportcard.com/report/github.com/carlosrabelo/negev)

**Negev** is a VLAN automation tool for Cisco switches over Telnet or SSH. It dynamically assigns VLANs based on MAC address prefixes, manages switch configurations, and keeps your playbook in sync with what is connected on each interface.

## Features

- **Telnet Management**: Connects to Cisco switches via Telnet to retrieve MAC address tables and configure VLANs.
- **SSH Management**: Connects to Cisco switches via SSH when Telnet is disabled or undesired.
- **MAC-Based VLAN Assignment**: Assigns VLANs based on the first three bytes of MAC addresses, with a default VLAN for unmapped devices.
- **Sandbox Mode**: Simulates configuration changes without applying them to the switch.
- **Configuration Persistence**: Saves changes to the switch's running configuration (with `-w` flag).
- **MAC Exclusion**: Ignores specified MAC addresses during VLAN assignment.
- **Port Exclusion**: Lets you skip interfaces that should never be touched.
- **Trunk Interface Detection**: Automatically skips trunk interfaces to avoid misconfiguration.
- **VLAN Creation**: Optionally creates missing VLANs on the switch (with `-c` flag).
- **Verbose Logging**: Provides detailed debug output for troubleshooting (use `-v 1`).
- **Raw Output Display**: Shows raw switch outputs for debugging (use `-v 2` or `-v 3`).
- **VLAN Validation**: Optionally skips VLAN existence checks (with `-s` flag).

## User Manuals

- [User manual (English)](docs/user_manual_en.md)
- [Manual do usuário (Português)](docs/user_manual_pt.md)

## Installation

Clone the repository and build the tool using the following commands

```bash
git clone https://github.com/carlosrabelo/negev.git
cd negev
go build -o negev ./cmd/negev
```

Or rely on the Makefile helpers (recommended):

```bash
make build
./bin/negev -t 192.168.1.1
```

## Configuration

The configuration is defined in a YAML file, specifying the default VLAN, MAC-to-VLAN mappings, and exclusions. A full sample lives in `configs/example.yaml`. Below is an excerpt:

```bash
transport: "telnet"
username: "admin"
password: "password"
enable_password: "enable_password"
default_vlan: "10"
no_data_vlan: "99"
exclude_macs:
  - "d8:d3:85:d7:0d:b7"
  - "ac:16:2d:34:bb:da"
mac_to_vlan:
  "3c:2a:f4": "30"  # Brother
  "dc:c2:c9": "30"  # Canon
  "00:c8:8b": "50"  # Cisco AP
switches:
  - target: "192.168.1.1"
    transport: "ssh"
    username: "admin"
    password: "password"
    enable_password: "enable_password"
    default_vlan: "10"
    no_data_vlan: "99"
    exclude_macs:
      - "00:11:22:33:44:55"
    exclude_ports:
      - "gi1/0/24"
    mac_to_vlan:
      "a4:bb:6d": "20"  # Custom device
```

#### Required Global Fields:

- **transport (optional)** Global transport for switch sessions (`telnet` by default, accepts `ssh`).
- **username, password, enable_password** Default credentials for switches (used if not specified per switch).
- **default_vlan** Global default VLAN for unmapped MACs.
- **no_data_vlan** Global quarantine VLAN for disconnected devices.
- **exclude_macs (optional)** List of full MAC addresses to ignore.
- **mac_to_vlan (optional)** Mapping of MAC prefixes (first 3 bytes) to VLANs.

#### Per-Switch Fields:

- **target** IP address of the Cisco switch.
- **transport (optional)** Overrides the global transport (`telnet` or `ssh`).
- **username, password, enable_password (optional)** Switch-specific credentials (falls back to global).
- **default_vlan (optional)** Switch-specific default VLAN (falls back to global).
- **no_data_vlan (optional)** Switch-specific quarantine VLAN (falls back to global).
- **exclude_macs (optional)** Switch-specific MACs to ignore (merged with global).
- **exclude_ports (optional)** List of interfaces to skip (comparison is case-insensitive).
- **mac_to_vlan (optional)** Switch-specific MAC-to-VLAN mappings (merged with global).

## ⚠️ Security

- **Telnet** Telnet is insecure and transmits credentials in plain text. Use only on trusted networks.
- **Sandbox Mode** Always test in sandbox mode (default) before applying changes with -w.
- **Credentials** Store sensitive information (username, password, enable_password) securely.

## Limitations

- **Transport** Telnet is the default; SSH support depends on the device having an interactive CLI similar to Telnet.
- **Single Switch** Each execution processes one switch (specified with `-t`).
- **No Reversion** Changes are not automatically reverted in case of failure.
- **Single MAC per Port** Ports with multiple MAC addresses are skipped to avoid ambiguity.
- **Switch Output Parsing** The tool assumes standard Cisco switch output formats; unexpected formats may cause parsing errors.

## Project Layout

- `cmd/negev`: CLI entry point and flag handling
- `internal/config`: YAML parsing and configuration validation
- `internal/transport`: Telnet/SSH transport clients with caching
- `internal/switchmanager`: VLAN orchestration logic over the transport layer
- `docs/`: User manuals in English (`user_manual_en.md`) and Portuguese (`user_manual_pt.md`)
- `bin/`: Build artifacts generated by `make build`

## Contributing

Contributions are welcome! Please submit issues or pull requests to the GitHub repository.
