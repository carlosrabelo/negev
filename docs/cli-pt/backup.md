# `--backup`

```text
--backup
```

Força backup do running-config mesmo no sandbox (sem `--write`). Com `--write`, o backup já ocorre antes das mutações.

## Exemplos

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml --backup
negev --target 192.0.2.10 --config docs/examples/config.yaml --write
```

## Notas

Arquivos em modo `0600` como `<nome-ou-ip>_<UTC>.cfg`. Falha de backup aborta antes de comandos mutáveis. Veja também [`backup-dir.md`](backup-dir.md).
