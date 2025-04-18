# `--backup`

```text
--backup
```

Força backup do running-config mesmo no sandbox (sem `--write`). Com `--write`, o backup só ocorre antes das mutações quando um diretório está configurado (`backup_dir` ou `--backup-dir`). `--backup` sem diretório resulta em erro.

## Exemplos

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml --backup
negev --target 192.0.2.10 --config docs/examples/config.yaml --write
```

## Notas

Arquivos em modo `0600` como `<nome-ou-ip>_<UTC>.cfg`. Falha de backup aborta antes de comandos mutáveis. Veja também [`backup-dir.md`](backup-dir.md).
