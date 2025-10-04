# Negev User Manual (English)

This manual explains how to operate **Negev** once the binary is available. It focuses on day-to-day usage, configuration scenarios, and troubleshooting.

## 1. Quick Overview
- Negev automates VLAN assignments on Cisco switches using Telnet or SSH.
- Decisions are based on MAC address prefixes, explicit exclusions, and default VLAN rules.
- By default, Negev works in *sandbox mode* (simulation only). Add `-w` to apply changes.

## 2. Required Files
- **Binary**: `negev` (obtained from the GitHub release or other distribution).
- **Configuration**: YAML file describing switches, credentials, VLAN mappings, and exclusions. Example provided at `configs/example.yaml`.

Place `config.yaml` next to the binary or in one of these locations:
- Current directory (`./config.yaml`).
- Linux: `~/.config/negev/config.yaml` or `/etc/negev/config.yaml`.
- Windows: `%APPDATA%\negev\config.yaml` or `%ProgramData%\negev\config.yaml`.

## 3. Understanding the Configuration File
Each section dictates how Negev should behave.

### 3.1 Global Settings
```yaml
transport: "ssh"        # default protocol for switches (telnet or ssh)
username: "admin"       # fallback username
password: "secret"       # fallback password
enable_password: "enable" # enable mode password
default_vlan: "10"       # VLAN used when no MAC prefix matches
no_data_vlan: "99"       # VLAN applied when a port becomes empty
exclude_macs:
  - "00:11:22:33:44:55"  # fully excluded MACs
mac_to_vlan:
  "aa:bb:cc": "20"      # prefix → VLAN mapping
```

- Global credentials act as defaults for switches that do not define their own.
- `mac_to_vlan` keys must be three-byte prefixes (six hex characters) and determine target VLANs.
- `exclude_macs` uses full MAC addresses; excluded devices are ignored.

### 3.2 Switch-Specific Overrides
```yaml
switches:
  - target: "192.168.1.10"
    transport: "telnet"
    username: "switch-admin"
    password: "switch-pass"
    enable_password: "switch-enable"
    default_vlan: "30"
    no_data_vlan: "88"
    exclude_macs:
      - "d8:d3:85:d7:0d:b7"
    exclude_ports:
      - "Gi1/0/24"
    mac_to_vlan:
      "dc:c2:c9": "50"
      "00:c8:8b": "70"
```

- `target` is mandatory and must match the `-t` flag when running Negev.
- `transport`, credentials, and VLAN settings override global values only for that switch.
- `exclude_ports` (case-insensitive) prevents Negev from touching sensitive interfaces.
- Merge logic: switch mappings take precedence over global mappings for the same prefix.

### 3.3 Best Practices for YAML
- Use lowercase for MAC prefixes and full addresses.
- Indentation must be two spaces (no tabs).
- Keep comments to explain unusual mappings or exclusions.
- Store credentials securely if the config is shared (e.g., use environment variable templates or external secrets manager).

## 4. Running Negev
Basic command:
```bash
negev -t 192.168.1.10
```

### Useful Flags
- `-y path/file.yaml` use a custom configuration path.
- `-w` apply changes instead of simulating.
- `-v 1` enable debug logs (merge decisions, exclusions).
- `-v 2` show raw switch output.
- `-v 3` enable both debug and raw logs.
- `-s` skip VLAN existence check (risky; use only if you know the VLAN is missing intentionally).
- `-c` create VLANs that are required by the mappings but absent on the switch.

### Typical Workflow
1. Run in sandbox mode to review changes:
   ```bash
   negev -t 192.168.1.10 -v 1
   ```
2. If output looks correct, apply changes:
   ```bash
   negev -t 192.168.1.10 -w -v 1
   ```

### Reading the Output
- `SANDBOX: ...` lines show the commands that would run.
- `Configured Gi1/0/1 to VLAN 20` confirms a successful change.
- `Warning: Multiple MACs detected on port ...` means Negev skipps that port to avoid mistakes.
- `Error: VLAN 50 does not exist on the switch` indicates the VLAN must be created (use `-c` if appropriate).

## 5. Maintaining Configuration
- Keep the YAML in version control without real passwords (use placeholders or environment variables).
- Update `mac_to_vlan` entries when adding new device types.
- Periodically review `exclude_macs` and `exclude_ports` to avoid stale entries.
- Share configuration snippets with teammates to maintain consistency across switches.

## 6. Troubleshooting
| Symptom | Possible Cause | Suggested Action |
| --- | --- | --- |
| "target ... not registered" | Missing switch entry | Ensure the `switches` list contains the target IP and correct casing |
| "No devices found" | Switch not reporting MAC table | Verify the switch supports the command; try `-v 2` to inspect raw output |
| SSH errors | Credentials or host key issues | Confirm login data, allow SSH, or switch to Telnet temporarily |
| VLAN mismatch keeps returning | Switch may revert changes or multiple devices exist | Check for trunk ports, port security, or multiple MACs warning |

## 7. Frequently Asked Questions
- **Does Negev support multiple switches per run?** No. Run once per switch using `-t` with the desired target.
- **Can I dry-run even with `-w` set?** No. Without `-w`, Negev always simulates. Use two runs: one sandbox, one real.
- **How does Negev treat trunk ports?** It automatically detects them and skips adjustments.
- **What happens to ports with more than one MAC?** They are ignored to avoid guessing the correct device.

## 8. Safety Checklist Before Applying Changes
- Are the required VLANs already present? If not, consider `-c`.
- Are all critical ports added to `exclude_ports`?
- Did you review mappings for typos (`aa:bb:c1` vs `aa:bb:c1`)?
- For sensitive environments, run with `-v 3` during the first deployment.

Following this guide helps you keep VLAN assignments tidy while avoiding surprises. Adjust the configuration iteratively, monitor logs, and enjoy a more automated network routine.
