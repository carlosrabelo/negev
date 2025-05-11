# Negev

[![Go Report Card](https://goreportcard.com/badge/github.com/carlosrabelo/negev)](https://goreportcard.com/report/github.com/carlosrabelo/negev)

**Negev** is a VLAN automation tool for Cisco switches, supporting both Telnet and SNMP. It dynamically assigns VLANs based on MAC address prefixes, manages switch configurations, and processes SNMP traps for real-time VLAN updates. It offers a flexible and configurable solution for network administrators.

## 🚀 Features

- **Telnet Management**: Connects to Cisco switches via Telnet to retrieve MAC address tables and configure VLANs.
- **SNMP Trap Processing**: Listens for SNMP traps to dynamically configure VLANs based on MAC address changes.
- **MAC-Based VLAN Assignment**: Assigns VLANs based on the first three bytes of MAC addresses, with a default VLAN for unmapped devices.
- **Sandbox Mode**: Simulates configuration changes without applying them to the switch.
- **Configuration Persistence**: Saves changes to the switch's running configuration (with `-w` flag).
- **MAC Exclusion**: Ignores specified MAC addresses during VLAN assignment.
- **Trunk Interface Detection**: Automatically skips trunk interfaces to avoid misconfiguration.
- **VLAN Creation**: Optionally creates missing VLANs on the switch (with `-c` flag).
- **Verbose Logging**: Provides detailed debug output for troubleshooting (with `-v` flag).
- **Raw Output Display**: Shows raw switch outputs for debugging (with `-e` flag).
- **VLAN Validation**: Optionally skips VLAN existence checks (with `-s` flag).

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
server_ip: "192.168.1.100"
username: "admin"
password: "password"
enable_password: "enable_password"
snmp_community: "public"
snmp_port: 162
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
    username: "admin"
    password: "password"
    enable_password: "enable_password"
    default_vlan: "10"
    no_data_vlan: "99"
    exclude_macs:
      - "00:11:22:33:44:55"
    mac_to_vlan:
      "a4:bb:6d": "20"  # Custom device
```

#### Required Global Fields:

- **server_ip** IPv4 address for the SNMP trap listener.
- **username, password, enable_password** Default credentials for switches (used if not specified per switch).
- **default_vlan** Global default VLAN for unmapped MACs.
- **no_data_vlan** Global quarantine VLAN for disconnected devices.
- **snmp_community (optional)** SNMP community string (defaults to "public").
- **snmp_port (optional)** SNMP port for traps (defaults to 162).
- **exclude_macs (optional)** List of full MAC addresses to ignore.
- **mac_to_vlan (optional)** Mapping of MAC prefixes (first 3 bytes) to VLANs.

#### Per-Switch Fields:

- **target** IP address of the Cisco switch.
- **username, password, enable_password (optional)** Switch-specific credentials (falls back to global).
- **default_vlan (optional)** Switch-specific default VLAN (falls back to global).
- **no_data_vlan (optional)** Switch-specific quarantine VLAN (falls back to global).
- **exclude_macs (optional)** Switch-specific MACs to ignore (merged with global).
- **mac_to_vlan (optional)** Switch-specific MAC-to-VLAN mappings (merged with global).

## ⚠️ Security

- **Telnet** Telnet is insecure and transmits credentials in plain text. Use only on trusted networks.
- **SNMP** The tool uses SNMPv2c, which lacks encryption. Ensure the SNMP community string is secure.
- **Sandbox Mode** Always test in sandbox mode (default) before applying changes with -w.
- **Credentials** Store sensitive information (username, password, enable_password) securely.

## 📋 Limitations

- **Telnet Only** Currently supports only Telnet for switch management; SSH support is planned.
- **Single Switch** Outside daemon mode, only one switch can be processed per execution (specified with -t).
- **No Reversion** Changes are not automatically reverted in case of failure.
- **Single MAC per Port** Ports with multiple MAC addresses are skipped to avoid ambiguity.
- **Switch Output Parsing** The tool assumes standard Cisco switch output formats; unexpected formats may cause parsing errors.
- **SNMP Traps** Only processes cmnMacChangedNotification traps for MAC address changes.

## 📎 Contributing

Contributions are welcome! Please submit issues or pull requests to the GitHub repository.
