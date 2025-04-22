# Negev

[![Go Report Card](https://goreportcard.com/badge/github.com/carlosrabelo/negev)](https://goreportcard.com/report/github.com/carlosrabelo/negev)
[![codecov](https://codecov.io/gh/carlosrabelo/negev/branch/master/graph/badge.svg)](https://codecov.io/gh/carlosrabelo/negev)

CLI tool for automating VLAN assignments on network switches based on MAC address prefixes.

## Highlights

- Connect to switches via Telnet or SSH with automatic platform detection
- Read MAC address tables and map prefixes to VLANs from a YAML config
- Assign access VLANs to switch ports based on connected device MACs
- Sandbox mode shows changes without applying — use `--write` to execute
- Create and delete VLANs to match an allowed list with protected VLAN safety
- Support Cisco IOS and Datacom DmOS through a pluggable driver system

## Installation

### Build from Source

```bash
git clone https://github.com/carlosrabelo/negev.git
cd negev
make build
```

Install to `~/.local/bin`:

```bash
make install
```

## Configuration

Create `config.yaml` in the current directory. Negev also searches `~/.config/negev/` and `/etc/negev/`.

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

switches:
  - target: 192.168.1.10
    platform: ios
```

## Usage

```bash
negev --target 192.168.1.10          # sandbox (simulate)
negev --target 192.168.1.10 --write  # apply changes
negev --target 192.168.1.10 --create-vlans
```

### Flags

| Flag | Description |
|------|-------------|
| `--target <ip>` | Switch IP address (required) |
| `--config <path>` | Path to YAML config file |
| `--write` | Apply changes (sandbox by default) |
| `--verbose <0-3>` | 0=none, 1=debug, 2=raw, 3=both |
| `--create-vlans` | Create/delete VLANs to match allowed list |
| `--version` | Show version |

## Project Layout

```
cmd/negev/              # CLI entry point
internal/domain/        # Core entities, interfaces, and business logic
internal/application/   # Service orchestration
internal/infrastructure/ # Config loading, transport (Telnet/SSH), adapter
internal/platform/      # Platform drivers (ios, dmos)
bin/                    # Compiled binary (git-ignored)
.make/                  # Build and install scripts
demos/                  # Sample configuration files
```

## Development

```bash
make build      # Compile binary to bin/negev
make test       # Run all tests
make quality    # Format, vet, and lint
make install    # Install to ~/.local/bin
```

## License

This project is licensed under the MIT License — see [LICENSE](LICENSE) for details.
