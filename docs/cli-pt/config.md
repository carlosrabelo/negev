# `--config`

```text
--config <path>
```

Caminho do arquivo YAML de configuração.

## Quando usar

Apontar para um arquivo fora do padrão (nos docs: `docs/examples/config.yaml`).

## Exemplos

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml
```

## Padrões

Se omitido, o Negev procura `config.yaml` em:

1. `./config.yaml`
2. `~/.config/negev/config.yaml` (Linux/macOS, diretório de config do usuário)
3. `/etc/negev/config.yaml` (Linux)
4. `%APPDATA%\negev\config.yaml` / `%ProgramData%\negev\config.yaml` (Windows)

Após resolver o caminho (via busca ou `--config`), stderr sempre imprime
`Using config: <path>`.

## Notas

Esquema completo e regras de mescla: [GUIDE-PT.md](../GUIDE-PT.md). Amostra:
[`docs/examples/config.yaml`](../examples/config.yaml).
