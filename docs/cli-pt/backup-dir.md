# `--backup-dir`

```text
--backup-dir <dir>
```

Diretório para backups de configuração. Sobrescreve o YAML `backup_dir`. Sem caminho padrão: se nem esta flag nem `backup_dir` estiverem definidos, o backup é omitido e nenhum diretório é criado.

## Exemplos

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml --write --backup-dir ./backups
negev --target 192.0.2.10 --config docs/examples/config.yaml --backup --backup-dir /tmp/negev-backups
```

## Padrões

Ordem: `--backup-dir` → YAML `backup_dir`. Vazio significa backups desabilitados.

## Notas

Veja também [`backup.md`](backup.md). Arquivos em modo `0600` como `<nome-ou-ip>_<UTC>.cfg`. O diretório só é criado quando um backup de fato ocorre.
