# Negev

CLI that automates VLAN assignments on access switches from MAC address prefixes.
It connects over Telnet or SSH, reads the MAC table, and configures access ports
from a YAML policy. Layout follows hexagonal / DDD boundaries
(`domain` → `application` → `infrastructure` / `platform` drivers).

Portuguese: [README-PT.md](README-PT.md).

## Highlights

- Multi-vendor drivers: Cisco IOS, Cisco NX-OS, Datacom DmSwitch 3000, Datacom DM1200E, 3Com/H3C Comware (`auto` detection)
- Safe by default: sandbox unless `--write`; protected VLANs (including 1000–4094)
- Dual transport: SSH (preferred) and Telnet with shared client cache
- Secrets outside YAML: `${ENV}` expansion, `password_command`, and `--ask-pass`
- Multi-switch runs: names, tags, `--all` / `--group`, bounded `--concurrency`
- Longest-prefix `mac_to_vlan`, `multi_mac_policy`, backups, and JSONL audit logging

## Prerequisites

- **Go 1.22.3+** — required to build from source; [download](https://go.dev/dl/)

## Installation

```bash
git clone https://github.com/carlosrabelo/negev.git
cd negev
make build
make install              # ~/.local/bin
# make install-system     # /usr/local/bin
```

Or:

```bash
go install github.com/carlosrabelo/negev/negev/cmd/negev@latest
```

Full docs: [docs/README.md](docs/README.md).

## Usage

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml
negev --target 192.0.2.10 --config docs/examples/config.yaml --write
negev --all --config docs/examples/config.yaml --concurrency 4
negev --group access --config docs/examples/config.yaml
```

Exit codes: `0` success / no pending sandbox changes, `1` error, `2` sandbox simulated changes.

Flags: [docs/cli/](docs/cli/) · Portuguese: [docs/cli-pt/](docs/cli-pt/) · guide: [docs/GUIDE.md](docs/GUIDE.md)

## Configuration

Minimal snippet (full schema in the [guide](docs/GUIDE.md); loadable sample:
[`docs/examples/config.yaml`](docs/examples/config.yaml)):

```yaml
platform: auto
transport: ssh
username: "${SWITCH_USER}"
password_command: "pass show lab/access"
default_vlan: "1"
mac_to_vlan:
  "aabbcc": "10"
switches:
  - name: access-a1
    target: 192.0.2.10
    platform: ios
    tags: [access]
```

## Project Layout

```
negev/cmd/negev/                 # CLI entry point
negev/internal/                  # domain, application, infrastructure, platform drivers
docs/                            # GUIDE, cli/, cli-pt/, examples/
.make/                           # Build and install scripts
```

## Development

```bash
make build             # Compile binary to bin/negev
make test              # Run all tests (local .gocache)
make quality           # Format and vet
make install           # Install binary to ~/.local/bin
make install-system    # Install binary to /usr/local/bin
make uninstall         # Remove from ~/.local/bin
make uninstall-system  # Remove from /usr/local/bin
```

## License

This project is licensed under the GNU General Public License v3.0 — see [LICENSE](LICENSE) for details.
