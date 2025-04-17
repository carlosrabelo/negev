# `--backup`

```text
--backup
```

Force a running-config backup even in sandbox mode (without `--write`). With `--write`, backup already runs before mutations.

## Examples

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml --backup
negev --target 192.0.2.10 --config docs/examples/config.yaml --write
```

## Notes

Files are written mode `0600` as `<name-or-ip>_<UTC>.cfg`. Backup failure aborts before any mutating command. See also [`backup-dir.md`](backup-dir.md).
