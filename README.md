# Negev

CLI that automates VLAN assignments on access switches from MAC address prefixes.
It connects over Telnet or SSH, reads the MAC table, and configures access ports
using a YAML policy. Architecture follows hexagonal / DDD boundaries
(`domain` → `application` → `infrastructure` / `platform` drivers).

## Highlights

- Multi-vendor drivers: Cisco IOS, Cisco NX-OS, Datacom DmSwitch 3000, Datacom DM1200E, 3Com/H3C Comware (`auto` detection supported)
- Safe by default: sandbox simulation unless `--write`; protected VLANs (including 1000–4094); VLAN 1 never deleted as a protected default range peer
- Dual transport: SSH (preferred) and Telnet with shared client cache
- Secrets outside YAML: `${ENV}` expansion, `password_command`, and `--ask-pass`
- Multi-switch runs: names, tags, `--all` / `--group`, bounded `--concurrency`
- Longest-prefix `mac_to_vlan` matching and configurable `multi_mac_policy`
- Pre-write running-config backups and JSONL audit logging

## Prerequisites

- **Go 1.22.3+** — required to build from source; [download](https://go.dev/dl/)

## Installation

### Build from Source

```bash
git clone https://github.com/carlosrabelo/negev.git
cd negev
make build
```

Install to `~/.local/bin` (default), or system-wide to `/usr/local/bin` (sudo only for the copy):

```bash
make install
make install-system
make uninstall
make uninstall-system
```

### Using Go Install

```bash
go install github.com/carlosrabelo/negev/negev/cmd/negev@latest
```

## Usage

Sandbox first (documentation IPs only):

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml
negev --target 192.0.2.10 --config docs/examples/config.yaml --write
negev --target 192.0.2.10 --config docs/examples/config.yaml --create-vlans --write
negev --all --config docs/examples/config.yaml --concurrency 4
negev --group access --config docs/examples/config.yaml
```

Exit codes: `0` success / no pending sandbox changes, `1` error, `2` sandbox simulated changes.

### Flags

| Flag | Description |
|------|-------------|
| `--target <ip\|name>[,...]` | Switch IP(s) and/or configured name(s), comma-separated |
| `--all` | Run against every switch in the config |
| `--group <tag>` | Run against switches that include the tag |
| `--concurrency <n>` | Max concurrent switch runs (default 4) |
| `--config <path>` | Path to YAML config file (default search: `./config.yaml`, user/system paths) |
| `--write` | Apply changes (sandbox/dry-run by default) |
| `--verbose <0-3>` | 0=none, 1=debug, 2=raw switch I/O, 3=both |
| `--create-vlans` | Create missing / delete extra VLANs vs `allowed_vlans` |
| `--ask-pass` | Prompt for switch / enable passwords |
| `--backup` | Backup running-config even in sandbox |
| `--backup-dir <dir>` | Backup directory (default `./backups` or YAML `backup_dir`) |
| `--audit-log <path>` | Append JSONL audit records (overrides YAML `audit_log`) |
| `--version` | Show version |

Full walkthroughs: [docs/GUIDE.md](docs/GUIDE.md) · [docs/GUIDE-PT.md](docs/GUIDE-PT.md)

## Configuration

Minimal example (see also [`docs/examples/config.yaml`](docs/examples/config.yaml)):

```yaml
platform: auto
transport: ssh
username: "${SWITCH_USER}"
password_command: "pass show lab/access"
enable_password: "${SWITCH_ENABLE}"
default_vlan: "1"
no_data_vlan: "999"
multi_mac_policy: ignore
backup_dir: ./backups
audit_log: ./audit/negev.jsonl
allowed_vlans: ["10", "20", "30"]
protected_vlans: ["999"]
mac_to_vlan:
  "aabbcc": "10"
  "aabbccddeeff": "15"   # longer prefix wins
switches:
  - name: access-a1
    target: 192.0.2.10
    platform: ios
    tags: [access, building-a]
  - name: access-a2
    target: 192.0.2.20
    platform: nxos
    transport: ssh
    tags: [access]
```

Supported `platform` values: `ios`, `nxos`, `dmswitch` (legacy alias `dmos`), `dm1200`, `comware`, `auto`.

## Project Layout

```
negev/cmd/negev/                 # CLI entry point
negev/internal/domain/           # Entities, ports, VLAN service
negev/internal/application/      # RunTarget, multi-runner, backup wiring
negev/internal/infrastructure/   # Config, transport, audit logger
negev/internal/platform/         # Drivers: ios, nxos, dmswitch, dm1200, comware
docs/                            # GUIDE + examples
bin/                             # Compiled binaries (git-ignored)
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
