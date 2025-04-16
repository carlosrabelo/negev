# Negev User Guide

Negev is a CLI for automating VLAN assignments on access switches from MAC address
prefixes. It connects via Telnet or SSH, reads the MAC table, resolves target VLANs
from YAML policy, and configures access ports. Hexagonal / DDD layout keeps domain
logic free of socket I/O; platform drivers and transport adapters sit at the edges.

English is the source of truth for this guide. Portuguese: [GUIDE-PT.md](GUIDE-PT.md).

## Architecture

| Layer | Path | Role |
|---|---|---|
| CLI | `negev/cmd/negev` | Flags, exit codes, `--ask-pass` |
| Application | `negev/internal/application` | `RunTarget`, multi-runner, backup orchestration |
| Domain | `negev/internal/domain` | Entities, ports, VLAN plan-then-apply service |
| Infrastructure | `negev/internal/infrastructure` | YAML loader, Telnet/SSH, audit JSONL |
| Platform | `negev/internal/platform` | Driver registry + IOS / NX-OS / DmSwitch / DM1200 / Comware |

Flow for one target: load/merge config → resolve driver (`auto` uses `show version` /
equivalent) → optional pre-write backup → `ProcessPorts()` → optional save → audit
append per port change.

Transport clients are cached by SHA-256 of `{Transport, Target, Username, Password, EnablePassword}`.
Telnet sends credentials in cleartext (warns). SSH uses a PTY and skips host-key
verification (warns)—use only on trusted networks.

## Installation

```bash
make install          # ~/.local/bin/negev
make install-system   # /usr/local/bin/negev
```

Or `go install github.com/carlosrabelo/negev/negev/cmd/negev@latest`.

## Configuration

Search order when `--config` is omitted and the file name is `config.yaml`:

- `./config.yaml`
- `~/.config/negev/config.yaml` (Linux/macOS via user config dir)
- `/etc/negev/config.yaml` (Linux)
- `%APPDATA%\negev\config.yaml` / `%ProgramData%\negev\config.yaml` (Windows)

After resolving the path (via search or `--config`), Negev always prints `Using config: <path>` on stderr.
Override search root in tests with `XDG_CONFIG_HOME` / `HOME` / `APPDATA` as needed.
Optional YAML `backup_dir` and `audit_log` apply globally.

### YAML schema (global + switch)

```yaml
platform: auto                 # ios | nxos | dmswitch | dm1200 | comware | auto
                               # legacy alias: dmos → dmswitch
transport: ssh                 # telnet | ssh
username: "${SWITCH_USER}"     # ${VAR} expanded at Load; $${VAR} → literal ${VAR}
password: ""                   # optional if password_command / --ask-pass
password_command: "pass show lab/sw"
enable_password: ""
enable_password_command: "pass show lab/enable"
default_vlan: "1"              # required
no_data_vlan: "999"            # optional quarantine for link-up ports with no MAC
multi_mac_policy: ignore       # ignore | alert | priority
backup_dir: ./backups
audit_log: ./audit/negev.jsonl
allowed_vlans: ["10", "20", "30"]
protected_vlans: ["999"]
exclude_macs: ["00:11:22:33:44:55"]
mac_to_vlan:
  "aabbcc": "10"               # even-length hex prefixes, 2..12 chars
  "aabbccddeeff": "15"         # longer match wins

switches:
  - name: access-a1            # optional, unique
    target: 192.0.2.10         # required
    tags: [access, building-a]
    platform: ios
    transport: ssh
    # per-switch overrides for credentials, VLANs, mac_to_vlan, exclude_*, multi_mac_policy
    exclude_ports: ["Gi1/0/24"]
    mac_to_vlan:
      "aabbcc": "0"            # remove inherited prefix for this switch
```

### Merging rules

1. **mac_to_vlan**: global + switch; switch wins on same key; VLAN `"0"` / `"00"` / `""` deletes that key at switch level.
2. **exclude_macs**: union, normalized, deduplicated (full 12-hex MACs).
3. **exclude_ports**: switch-only; trimmed, lowercased, deduplicated.
4. **allowed_vlans / protected_vlans**: merged lists, canonical VLAN strings.
5. **multi_mac_policy**: switch inherits global; default `ignore`.
6. **Credentials**: empty switch fields inherit global (after env expansion and password commands).

Sample file: [`docs/examples/config.yaml`](examples/config.yaml).

### Secrets

- `${VAR}` in `username` / `password` / `enable_password` (missing var → Load error).
- `password_command` / `enable_password_command` when the password field is empty: `/bin/sh -c` (or `cmd /C`), 5s timeout, minimal env, trim trailing `\r\n`.
- `--ask-pass` prompts once and overrides YAML/env for the selected target(s).

## CLI reference

```bash
negev (--target <ref>[,...] | --all | --group <tag>) [flags]
```

`--target`, `--all`, and `--group` are mutually exclusive.

| Flag | Description |
|---|---|
| `--target <ip\|name>[,...]` | IP(s) and/or switch `name`(s) |
| `--all` | Every switch in the file |
| `--group <tag>` | Switches whose `tags` contain `<tag>` |
| `--concurrency <n>` | Worker pool size (default 4) |
| `--config <path>` | YAML path |
| `--write` | Apply changes (default = sandbox) |
| `--verbose <0-3>` | 0 none, 1 debug, 2 raw I/O, 3 both |
| `--create-vlans` | Sync VLANs to `allowed_vlans` |
| `--ask-pass` | Interactive passwords |
| `--backup` | Backup running-config even in sandbox |
| `--backup-dir <dir>` | Backup directory |
| `--audit-log <path>` | JSONL audit path (overrides YAML) |
| `--version` | Version string |

Exit codes: `0` OK / no pending sandbox changes, `1` error on any target, `2` sandbox simulated changes and no failures.

```bash
negev --all --config docs/examples/config.yaml --concurrency 4
negev --group access --config docs/examples/config.yaml
negev --target access-a --config docs/examples/config.yaml --ask-pass
negev --version
```

Multi-run output is buffered per target and flushed with a `[label]` prefix; a summary table prints at the end.

## Supported platforms

| Platform | YAML `platform` | Discover | Access VLAN | Save / backup dump |
|---|---|---|---|---|
| Cisco IOS | `ios` | `show mac address-table`, `show interfaces status` | `switchport access vlan` | `write memory` / `show running-config` |
| Cisco NX-OS | `nxos` | `show mac address-table`, `show interface status` | `switchport` then `switchport access vlan` | `copy running-config startup-config` / `show running-config` |
| Datacom DmSwitch 3000 | `dmswitch` (`dmos`) | `show mac-address-table`, switchport status | `set-member untagged` / native VLAN style cmds | `show running-config` |
| Datacom DM1200E | `dm1200` | `show mac-addr-table` | `vlan database`, `switchport access vlan`, `vlan participation exclude 1` | `show running-config` |
| 3Com / H3C Comware | `comware` | `display mac-address`, `display brief interface` | `port access vlan` | `save` / `display current-configuration` |

`platform: auto` probes version output alphabetically across registered drivers (Comware, DM1200, DmSwitch, IOS, NX-OS). IOS detection rejects NX-OS banners.

## How it works

1. Resolve selected switches (`ResolveTargets`).
2. Per switch (concurrent pool): load credentials, detect/get driver, set session profile + auth sequence.
3. If `--write` or `--backup`: download running-config and write `<name-or-ip>_<UTC>.cfg` mode `0600` under `--backup-dir` / `backup_dir` / `./backups`. Backup failure aborts before any mutating command.
4. Collect VLANs, trunks, L3/SVI interfaces, active ports, MAC table.
5. Skip trunks, routed ports, `exclude_ports`, excluded MACs.
6. Resolve VLAN: longest even-length prefix in `mac_to_vlan` (12→2); else `default_vlan`; empty ports may use `no_data_vlan`.
7. Multi-MAC ports: `ignore` (skip quiet), `alert` (warn + skip), `priority` (longest explicit mapping wins).
8. Optional `--create-vlans`: create missing allowed; delete non-allowed non-protected (1000–4094 always protected).
9. Sandbox prints `SIMULATE:`; `--write` executes and saves.
10. Each successful or failed port VLAN change appends one JSONL audit record when `audit_log` / `--audit-log` is set (mode `0644`, sync per entry).

## Multi-MAC policy

| Policy | Behavior |
|---|---|
| `ignore` (default) | Skip ports with more than one active MAC; no warning |
| `alert` | Skip and log every MAC with its resolved VLAN |
| `priority` | Among explicit `mac_to_vlan` hits, pick the longest prefix; if none, skip |

## Backup and audit

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml --write --backup-dir ./backups
negev --target 192.0.2.10 --config docs/examples/config.yaml --backup   # sandbox + force backup
negev --target 192.0.2.10 --config docs/examples/config.yaml --write --audit-log ./audit/negev.jsonl
```

Audit fields: `timestamp`, `user`, `target`, `switch_name`, `port`, `mac`, `old_vlan`, `new_vlan`, `action` (`SET_PORT_VLAN`), `sandbox`, `status`.

## MAC normalization

Separators `:`, `.`, `-` are stripped; letters lowercased. Prefix keys in `mac_to_vlan` must be even length 2–12 hex. Exclude lists require full 12-character MACs after normalization.

## Practical scenarios

### Scenario 1: Initial access-switch deployment

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml
negev --target 192.0.2.10 --config docs/examples/config.yaml --write
```

### Scenario 2: Synchronize the VLAN database

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml --create-vlans
negev --target 192.0.2.10 --config docs/examples/config.yaml --create-vlans --write
```

### Scenario 3: Segment unmanaged access ports

Map department prefixes (and exact MACs) in `mac_to_vlan`, list uplinks in `exclude_ports`, set `multi_mac_policy` for phone+PC ports, preview, then `--write`.

### Scenario 4: Troubleshoot prompts and pagers

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml --verbose 1
negev --target 192.0.2.10 --config docs/examples/config.yaml --verbose 2
negev --target 192.0.2.10 --config docs/examples/config.yaml --verbose 3
negev --target 192.0.2.10 --config docs/examples/config.yaml --ask-pass --verbose 1
```

Use `--ask-pass` or `password_command` when credentials must not sit in YAML.
