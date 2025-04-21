# `--backup-dir`

```text
--backup-dir <dir>
```

Directory for configuration backups. Overrides YAML `backup_dir`.

## Examples

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml --write --backup-dir ./backups
negev --target 192.0.2.10 --config docs/examples/config.yaml --backup --backup-dir /tmp/negev-backups
```

## Defaults

Resolution order: `--backup-dir` → YAML `backup_dir` → `./backups`.

## Notes

See also [`backup.md`](backup.md). Files are mode `0600` as `<name-or-ip>_<UTC>.cfg`.
