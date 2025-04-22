# Negev

## First milestone

VLAN automation for Cisco IOS switches over Telnet/SSH. MAC-based assignment with YAML config, sandbox mode, VLAN sync.

## Foundation

- [x] Go module layout (`negev/cmd/` + `negev/internal/`) with hexagonal architecture (domain/ports/services → application → infrastructure → platform)
- [x] Build system: Makefile with `build`, `test`, `fmt`, `lint`, `quality`, `install`, `uninstall`, `run`, `clean`, `deps`, `info`, `version`, `help`
- [x] Build scripts in `.make/`: `build.sh`, `test.sh`, `install.sh`, `uninstall.sh`, `run.sh`, `clean.sh`
- [x] Go 1.22 toolchain, local `GOCACHE=.gocache`, ldflags injection for `version` and `buildTime`
- [x] YAML config loading with global → switch merge: `debugf` helper for conditional debug output
- [x] Backward compat: `vendor` → `platform`, `LegacyPlatform`/`LegacyVendor` fallback
- [x] `NormalizeMAC`: strips `:` and `.`, lowercases to 12 hex chars
- [x] MAC prefix = first 6 chars of normalized MAC
- [x] Config path resolution: `./config.yaml`, `~/.config/negev/`, `/etc/negev/` (Linux); `%APPDATA%`, `%ProgramData%` (Windows)
- [x] `NoDataVlan` field (quarantine VLAN, reserved)
- [x] Config validation: required fields (`default_vlan`, `no_data_vlan`, `username`, `password`, `enable_password`), VLAN range 1–4094, platform enum (`ios`/`dmos`/`auto`), transport enum (`telnet`/`ssh`)
- [x] `mergeStringSlices`: generic merge with dedup and optional per-element validation
- [x] Per-switch `ExcludeMacs` merge: global + switch, normalized to 12 hex
- [x] Per-switch `MacToVlan` merge: switch-specific overrides global for same prefix; `"0"`/`"00"` VLAN ignored
- [x] Per-switch `ExcludePorts`: switch-only, normalized (trim, lowercase, dedup)
- [x] Per-switch verbose suppression: debug output only for the target switch
- [x] `Sandbox = !write` (inverted boolean)

## Domain entities

- [x] `SwitchConfig`: Platform, LegacyPlatform, Target, Transport, Username, Password, EnablePassword, MacToVlan, ExcludeMacs, ExcludePorts, DefaultVlan, NoDataVlan, AllowedVlans, ProtectedVlans, Sandbox, VerbosityLevel, CreateVLANs
- [x] `SwitchConfig.IsDebugEnabled()`: `VerbosityLevel == 1 || 3`
- [x] `SwitchConfig.IsRawOutputEnabled()`: `VerbosityLevel == 2 || 3`
- [x] `SwitchConfig.PlatformID()`: normalized platform, falls back to LegacyPlatform, default `"ios"`
- [x] `Device`: Vlan (string), Mac (12 hex lowercase), MacFull (`xx:xx:xx:xx:xx:xx`), Interface
- [x] `Port`: Interface, Vlan (string)
- [x] `AuthPrompt`: WaitFor (prompt to match), SendCmd (response to send)

## Domain ports (interfaces)

- [x] `VLANService` interface: `ProcessPorts()`, `GetVlanList()`, `GetTrunkInterfaces()`, `GetActivePorts()`, `GetMacTable()`, `ConfigureVlan()`, `CreateVLAN()`, `DeleteVLAN()`
- [x] `SwitchRepository` interface: `Connect()`, `Disconnect()`, `ExecuteCommand()`, `IsConnected()`

## Parse utilities (`internal/platform/parseutil/`)

- [x] `FormatPlainMac`: 12 hex chars → `"xx:xx:xx:xx:xx:xx"` format
- [x] `IsSeparatorLine`: detects lines of `-`, `=`, `+`, `*` (min 3 chars) or blank lines

## Transport

### Telnet (`internal/infrastructure/transport/telnet.go`)

- [x] Raw Telnet client on port 23
- [x] Security warning: "credentials are transmitted in cleartext"
- [x] Constants: `DefaultTimeout = 120s`, `BufferSize = 4096`, `PromptUsername = "Username:"`, `PromptPassword = "Password:"`, `PromptEnable = ">"`, `PromptPrivileged = "#"`, `TerminalLengthCmd = "terminal length 0\n"`
- [x] Custom authentication sequence via `SetAuthSequence`
- [x] Default IOS auth sequence when none set (Username → Password → enable → Password → terminal length 0 → #)
- [x] `Connect`: dials TCP, reads prompts and sends auth commands
- [x] `readUntil(pattern, timeout)`: reads chunks, sleeps 100ms between reads, checks for pattern match
- [x] `ExecuteCommand`: sends command + `\n`, reads until `#`, strips echo line and trailing prompt line
- [x] `Disconnect`: closes connection, sets conn to nil
- [x] `IsConnected`: checks `conn != nil`
- [x] Raw output display when `IsRawOutputEnabled`

### SSH (`internal/infrastructure/transport/ssh.go`)

- [x] SSH client on port 22 with `InsecureIgnoreHostKey()` (//nolint:gosec)
- [x] Security warning: "SSH host key verification is disabled"
- [x] `Dialer` with `DefaultTimeout` (120s)
- [x] PTY vt100 session with ECHO=0, ISPEED/OSPEED=9600
- [x] `Connect`: dials, creates session, requests PTY, starts shell, reads initial prompt, elevates to enable if needed, sends `terminal length 0`
- [x] `readUntilAny(patterns, timeout)`: buffered reader, `SetReadDeadline(500ms)` for non-blocking, retries on timeout until global deadline
- [x] `ExecuteCommand`: sends command + `\n`, reads until `#`, strips echo line and trailing prompt line
- [x] `Disconnect`: closes session, client, and raw net.Conn; nil-safe
- [x] `IsConnected`: checks `session != nil && client != nil`
- [x] Raw output display when `IsRawOutputEnabled`

### Client cache (`internal/infrastructure/transport/client.go`)

- [x] Global `clientCache map[string]Client` with `sync.Mutex`
- [x] `cacheKey(cfg)`: JSON-marshals `{Transport, Target, Username, Password, EnablePassword}` → SHA256 → hex
- [x] `Get(cfg)`: check cache → create via `newClient` → store → return
- [x] `newClient(cfg)`: SSH if `cfg.Transport == "ssh"`, otherwise Telnet
- [x] `CloseAll()`: iterate cache, `Disconnect()` each, delete from map

### SwitchAdapter (`internal/infrastructure/transport/switch_adapter.go`)

- [x] `Client` interface: `Connect()`, `Disconnect()`, `ExecuteCommand()`, `IsConnected()`
- [x] `AuthConfigurable` interface: `SetAuthSequence([]entities.AuthPrompt)`
- [x] `SwitchAdapter` wraps `Client` and implements `SwitchRepository`

## Platform driver interface

- [x] `SwitchDriver` contract: `Name()`, `Detect()`, `GetAuthenticationSequence()`, `GetVLANList()`, `GetTrunkInterfaces()`, `GetActivePorts()`, `GetMacTable()`, `ConfigureAccessCommands()`, `CreateVLANCommands()`, `DeleteVLANCommands()`, `SaveCommands()`
- [x] Global registry: `[]SwitchDriver` with `Get(name)`, `Available()`, `Detect(repo)`

## Cisco IOS (`internal/platform/ios/`)

- [x] Driver name: `"ios"`
- [x] Auth sequence: `Username:` → username, `Password:` → password, `>` → `enable`, `Password:` → enable password, `#` → `terminal length 0`, `#` → `""`
- [x] Detection: `show version` output contains `"cisco ios"` (case-insensitive)
- [x] VLAN list: `show vlan brief` with fallback to `show vlan`
- [x] VLAN list parser: `vlanLineRegex = ^\s*(?:vlan\s+)?(\d{1,4})\b`, skips separator lines and blanks
- [x] Trunk interfaces: `show interfaces trunk`, first field matched against `interfaceRegex = ^[A-Za-z]+\d+(?:/\d+){0,2}$`
- [x] Active ports: `show interfaces status`, status keywords: connected/up/forward/monitor/active/link-up, explicit `notconnect` exclusion, extracts VLAN from field after status, sorts by interface name
- [x] MAC table: `show mac address-table dynamic`, first fetches trunks to filter out
- [x] MAC table parser: `macTableRegex = ^\s*(\d+)\s+([0-9A-Fa-f]{4}\.[0-9A-Fa-f]{4}\.[0-9A-Fa-f]{4})\s+DYNAMIC\s+(\S+)`, validates MAC format, validates interface format, skips trunks
- [x] Config commands: `configure terminal`, `interface <iface>`, `switchport mode access`, `switchport access vlan <vlan>`, `end`
- [x] VLAN create: `configure terminal`, `vlan <id>`, `exit`, `interface vlan <id>`, `no shutdown`, `end`
- [x] VLAN delete: `configure terminal`, `interface vlan <id>`, `shutdown`, `exit`, `no interface vlan <id>`, `exit`, `no vlan <id>`, `end`
- [x] Save: `write memory`
- [x] Command error detection (`isIOSCommandError`): "invalid input", "unknown command", "incomplete command", "ambiguous command", "unrecognized command", "invalid command", "syntax error", "cannot find command"
- [x] Raw output display when `IsRawOutputEnabled` (via transport layer)

## VLAN service

- [x] `VLANServiceImpl` with compile-time interface check: `var _ ports.VLANService = (*VLANServiceImpl)(nil)`
- [x] Constructor: `NewVLANService(switchRepo, config, driver)`
- [x] `ProcessPorts` flow: connect → get VLAN list → sync VLANs (if `--create-vlans`) → get trunks → get active ports → get MAC table → iterate ports → map MAC → configure → save
- [x] Multiple MACs on same port → `log.Printf` warning and skip (safety)
- [x] Malformed MAC (< 6 chars after normalization) → skip
- [x] Target VLAN fallback: `MacToVlan[macPrefix]` → if empty/`"0"`/`"00"` → `DefaultVlan`
- [x] Non-existent target VLAN on switch → error and skip port
- [x] Excluded MACs (exact match against normalized 12-char list) → skip port
- [x] Excluded ports (case-insensitive match) → skip port
- [x] Protected VLANs: user-defined `ProtectedVlans` list + extended range (1000–4094) auto-protected
- [x] Trunk interface skip: detected via driver, case-insensitive match
- [x] Sandbox mode: prints simulated commands, no execution
- [x] VLAN sync (`--create-vlans`): create missing from `AllowedVlans`, delete extras (skip protected)
- [x] VLAN iteration uses `sortedKeys` (deterministic alphabetical order)
- [x] `ConfigureVlan`: delegates to `driver.ConfigureAccessCommands`, sandbox or execute
- [x] `CreateVLAN` / `DeleteVLAN`: delegates to driver, sandbox or execute
- [x] `saveConfiguration`: tries each `driver.SaveCommands()`, returns last error if all fail
- [x] `getAllowedVLANs`: builds `map[string]bool` from `AllowedVlans`
- [x] `filterDevices`: case-insensitive port match
- [x] `deviceMacs`: extract MAC list from devices
- [x] `isExcluded`: exact MAC match against list
- [x] Summary messages: "No changes required", "Changes simulated (sandbox mode, use -w to apply)"

## Application orchestration

- [x] `RunTarget` in `internal/application/services/runner.go`
- [x] Switch lookup by target in `cfg.Switches`; error if not found
- [x] `PlatformID()` resolves platform name, `"auto"` triggers `platform.Detect()` with actual connection
- [x] Auto-detect path: create client → adapter → detect → set auth sequence → reuse client → create service → process
- [x] Explicit platform path: `platform.Get(name)` → create client → set auth sequence → create service → process
- [x] `VLANApplicationService` facade: creates `SwitchAdapter` + `VLANServiceImpl`, delegates `ProcessPorts()`
- [x] Auth sequence injection per platform via `AuthConfigurable.SetAuthSequence()`

## CLI

- [x] `negev` entry point in `cmd/negev/main.go`
- [x] Global version/build output: `fmt.Printf("Negev %s (built %s)\n", version, buildTime)`
- [x] Flags: `--target <ip>` (required, validated against YAML), `--config <path>` (default `config.yaml`), `--write` (disables sandbox), `--verbose <0-3>` (0=none, 1=debug, 2=raw, 3=both), `--create-vlans` (sync VLANs)
- [x] `flag.Usage = printUsage` with flag descriptions
- [x] Validation: `--verbose` 0–3 range check, `--target` non-empty check
- [x] `defer transport.CloseAll()` on exit
- [x] Invalid arguments exit with usage text and os.Exit(1)
- [x] `FindPath` resolves config file location
- [x] `config.Load` validates and returns parsed `*Config`
- [x] `services.RunTarget` orchestrates the target switch processing

## Project artifacts

- [x] Sample configuration (`demos/config.yaml`) with global defaults and per-switch overrides, MAC-to-VLAN mappings, exclude lists, protected VLANs

## Documentation

- [x] User Guide in English (`docs/GUIDE.md`)
- [x] User Guide in Portuguese (`docs/GUIDE-PT.md`)

## CI/CD

- [x] GitHub Actions: lint, test, build
- [x] GitHub Actions: release automation with goreleaser
- [x] Go Report Card badge
- [x] Code coverage tracking

---

## Second milestone

Add support for Datacom DmOS switches.

## Datacom DmOS (`internal/platform/dmos/`)

- [x] Driver name: `"dmos"`, registered in global registry
- [x] Auth sequence: `login:` → username, `Password:` → password, `#` → `terminal length 0`, `#` → `""` (no enable command needed)
- [x] Detection: `show version` output contains `"dmos"` or `"datacom"` (case-insensitive)
- [x] VLAN list: `show vlan table` with fallback to `show vlan`
- [x] VLAN list parser: `^VLAN\s+(\d+)\s*(?:\[.*?\])?:\s*` (e.g. `VLAN 1 [DefaultVlan]:`), fallback to `vlanPrefixRegex`
- [x] Trunk interfaces: parsed from cached switchport output via `parseDmOSTrunksFromSwitchport`: looks for `(s,t)` tagged VLANs in "Allowed VLANs:" sections per interface
- [x] Alternative trunk parser `parseDmOSTrunks`: column-split table format with `dmosPortRegex`
- [x] Active ports: `show interfaces status` + `show interfaces switchport`; parses `Information of Eth X/Y` + `Link status: Up/Down`; enriches with VLAN from switchport output via `parseDmOSSwitchportVLANs` (parses `Native VLAN: <id>`); sorts by numeric unit/port
- [x] `compareInterfaceNames`: extracts `X/Y` numbers, compares unit first then port
- [x] Switchport output cache: thread-safe with `sync.Mutex`, keyed by target, avoids duplicate `show interfaces switchport` calls
- [x] MAC table: `show mac-address-table`
- [x] MAC parser: `macLineRegex = ^\s*\d+\s+\w*\s+(Eth\s+\d+/\d+)\s+([0-9A-F:]+)\s+(\d+)\s+.*Learned`; normalizes port to `"ethernet X/Y"`; skips trunks
- [x] `normalizeMac`: strips `.` and `:`, lowercases
- [x] `normalizePort`: prepends `"ethernet "` prefix if missing
- [x] Config commands: `configure`, `interface vlan <vlan>`, `set-member untagged <port>`, `exit`, `interface <port>`, `switchport native vlan <vlan>`, `switchport acceptable-frame-type all`, `exit`, `end`
- [x] VLAN create: `configure`, `interface vlan <id>`, `exit`, `end` (DmOS creates VLAN implicitly)
- [x] VLAN delete: `configure`, `no interface vlan <id>`, `end`
- [x] Save: `copy running-config startup-config` with fallback to `save`
- [x] Command error detection (`isDmOSCommandError`): "unknown command", "invalid", "incomplete", "syntax error"
- [x] Raw output display when `IsRawOutputEnabled`

---

## Enhancements

- [ ] Multi-switch batch processing (iterate all switches)
- [ ] Report generation (CSV, JSON, HTML)
- [ ] Auto-completion for bash/zsh
- [ ] Docker image and Compose example
- [ ] Config encryption for stored passwords
- [ ] Pre/post validation hooks
- [ ] Syslog/alerting integration on change
- [ ] Rolling update with staggered switch processing
