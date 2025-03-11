# Negev

Automates VLAN assignments on access switches from MAC address prefixes.


## Usage

```bash
negev --target 192.168.1.10                   # sandbox (simulate)
negev --target 192.168.1.10 --write           # apply changes
negev --target 192.168.1.10 --create-vlans --write # sync VLANs and apply
```

### Flags

| Flag | Description |
|------|-------------|
| `--target <ip>` | Switch IP address (required, must exist in config) |
| `--config <path>` | Path to YAML config file (default: config.yaml) |
| `--write` | Apply changes (sandbox/dry-run by default) |
| `--verbose <0-3>` | Output level: 0=none, 1=debug, 2=raw output, 3=both |
| `--create-vlans` | Create/delete VLANs to match allowed list |
| `--version` | Show version |

## Configuration

Create `config.yaml` in the current directory. Negev also searches `~/.config/negev/` and `/etc/negev/` (Linux), or `%APPDATA%\negev\` and `%ProgramData%\negev\` (Windows).

```yaml
platform: auto
transport: telnet
username: admin
password: cisco123
enable_password: cisco123
default_vlan: "1"
no_data_vlan: "999"
allowed_vlans:
  - "10"
  - "20"
switches:
  - target: 192.168.1.10
    platform: ios
```

## Development

```bash
make build             # Compile binary to bin/negev
make test              # Run all tests
make quality           # Format, vet, and lint
make install           # Install binary to ~/.local/bin
make install-system    # Install binary to /usr/local/bin
make uninstall         # Remove from ~/.local/bin
make uninstall-system  # Remove from /usr/local/bin
```

## License

This project is licensed under the GNU General Public License v3.0 — see [LICENSE](LICENSE) for details.
