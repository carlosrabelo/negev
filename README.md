# Negev

[![Go Report Card](https://goreportcard.com/badge/github.com/carlosrabelo/negev)](https://goreportcard.com/report/github.com/carlosrabelo/negev)

**Negev** is a VLAN automation tool for Cisco switches via Telnet. It dynamically assigns VLANs based on MAC address prefixes, offering a flexible and easy-to-configure solution.

## 🚀 Features

- Telnet connection to Cisco switches
- Device identification using the dynamic MAC address table
- Automatic VLAN assignment based on MAC prefixes
- Sandbox mode for safe simulation
- Configuration persistence with write memory
- Dynamic VLAN replacement via CLI
- Exclusion of specific MAC addresses from reconfiguration
- Automatic detection and exclusion of trunk interfaces
- Automatic creation of missing VLANs on the switch

## 🔧 Installation

Clone the repository and build the tool using the following commands

```bash
git clone https://github.com/carlosrabelo/negev.git
cd negev
go build -o negev main.go
```

## 📂 Configuration

The configuration is defined in a YAML file, specifying the default VLAN, MAC-to-VLAN mappings, and exclusions. Below is an example:

```bash
host: "192.168.1.1"
username: "admin"
password: "senha"
enable_password: "senha_enable"
default_vlan: "10"

mac_to_vlan:
  "3c:2a:f4": "30"  # Brother
  "dc:c2:c9": "30"  # Canon
  "00:c8:8b": "50"  # Cisco AP

exclude_macs:
  - "d8:d3:85:d7:0d:b7"
  - "ac:16:2d:34:bb:da"
```

Required fields:

- **host** (IP address of the Cisco switch)
- **username**/**password**/**enable_password** (Telnet and privileged mode credentials)
- **default_vlan** (used for unmapped MACs)
- **mac_to_vlan** (mapping of MAC prefixes, first 3 bytes, to VLANs)
- **exclude_macs** (full MAC addresses to ignore)

## 📌 Examples:

Run in sandbox mode:

`negev -y example.yaml`

Apply configurations to the switch:

`negev -y example.yaml -x`

Replace VLANs dynamically (e.g., VLAN 10 to 100):

 `negev -y example.yaml -x -r 10,100`

Run with debug output:

`negev -y example.yaml -x -d`

Skip VLAN validation:

`negev -y example.yaml -w -s`

Create missing VLANs:

`negev -y example.yaml -w -c`

Override the YAML host:

`negev -y example.yaml -h 10.0.0.1`

## ⚠️ Security

- Telnet is insecure; use only on trusted networks
- Negev applies changes without confirmation
- Test in sandbox mode (default) before using -w

📋 Limitations

- Uses Telnet (insecure); SSH support is planned.
- Supports only one switch per execution.
- Does not revert changes in case of failure.
- Assumes a single MAC address per port to avoid ambiguity in VLAN assignment. If multiple MACs are detected on a port, the port is skipped with a warning.
- Parsing of switch commands may fail with unexpected output formats.

## 📎 Contributing

Contributions are welcome! Please submit issues or pull requests to the GitHub repository.
