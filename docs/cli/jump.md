# `--jump`

```text
--jump <[user@]host[:port]|none>
```

Tunnel both Telnet and SSH switch connections through a single SSH jump host (bastion) using `direct-tcpip` forwarding. Overrides YAML `jump_host` for the selected targets.

## When to use

When switches sit on an isolated management network reachable only from a bastion host (for example over a VPN).

## Examples

```bash
negev --tag access --config docs/examples/config.yaml --jump ops@192.0.2.5
negev --target 192.0.2.10 --config docs/examples/config.yaml --jump ops@192.0.2.5:2222 --write
negev --target 192.0.2.10 --config docs/examples/config.yaml --jump none
```

## Notes

- Host key verification is mandatory against `~/.ssh/known_hosts` (override path with [`--jump-known-hosts`](jump-known-hosts.md)).
- Authentication tries `--jump-key` / `jump_host.key_file`, `ssh-agent`, default `~/.ssh/id_{ed25519,ecdsa,rsa}`, and password / `--jump-ask-pass`.
- Use `none` (or `off`, `direct`, `false`) to disable a YAML `jump_host` for a single run.
- One SSH connection per jump host is multiplexed across concurrent `--concurrency` switch workers.
