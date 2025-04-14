# Negev

CLI que automatiza atribuições de VLAN em switches de acesso a partir de prefixos MAC.
Conecta via Telnet ou SSH, lê a tabela MAC e configura portas de acesso com política YAML.
A arquitetura segue limites hexagonais / DDD
(`domain` → `application` → `infrastructure` / drivers `platform`).

## Destaques

- Drivers multi-vendor: Cisco IOS, Cisco NX-OS, Datacom DmSwitch 3000, Datacom DM1200E, 3Com/H3C Comware (detecção `auto`)
- Seguro por padrão: simulação sandbox sem `--write`; VLANs protegidas (incluindo 1000–4094)
- Transporte dual: SSH (preferencial) e Telnet com cache de clientes
- Segredos fora do YAML: expansão `${ENV}`, `password_command` e `--ask-pass`
- Execução multi-switch: nomes, tags, `--all` / `--group`, `--concurrency` limitado
- Matching `mac_to_vlan` por prefixo mais longo e `multi_mac_policy` configurável
- Backup do running-config antes de writes e auditoria JSONL

## Pré-requisitos

- **Go 1.22.3+** — necessário para compilar a partir do código-fonte; [download](https://go.dev/dl/)

## Instalação

### Compilar a partir do código-fonte

```bash
git clone https://github.com/carlosrabelo/negev.git
cd negev
make build
```

Instala em `~/.local/bin` (padrão), ou em todo o sistema em `/usr/local/bin` (sudo apenas para a cópia):

```bash
make install
make install-system
make uninstall
make uninstall-system
```

### Usando Go Install

```bash
go install github.com/carlosrabelo/negev/negev/cmd/negev@latest
```

## Uso

Sandbox primeiro (apenas IPs de documentação):

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml
negev --target 192.0.2.10 --config docs/examples/config.yaml --write
negev --target 192.0.2.10 --config docs/examples/config.yaml --create-vlans --write
negev --all --config docs/examples/config.yaml --concurrency 4
negev --group access --config docs/examples/config.yaml
```

Códigos de saída: `0` sucesso / sem mudanças pendentes no sandbox, `1` erro, `2` mudanças simuladas no sandbox.

### Flags

| Flag | Descrição |
|------|-----------|
| `--target <ip\|name>[,...]` | IP(s) e/ou nome(s) configurados, separados por vírgula |
| `--all` | Executa contra todos os switches da configuração |
| `--group <tag>` | Executa contra switches que incluem a tag |
| `--concurrency <n>` | Máximo de execuções concorrentes (padrão 4) |
| `--config <path>` | Caminho do YAML (busca padrão: `./config.yaml`, caminhos de usuário/sistema) |
| `--write` | Aplica alterações (sandbox/dry-run por padrão) |
| `--verbose <0-3>` | 0=nenhum, 1=debug, 2=I/O bruto do switch, 3=ambos |
| `--create-vlans` | Cria VLANs faltantes / remove extras vs `allowed_vlans` |
| `--ask-pass` | Solicita senhas de switch / enable interativamente |
| `--backup` | Faz backup do running-config mesmo no sandbox |
| `--backup-dir <dir>` | Diretório de backups (padrão `./backups` ou YAML `backup_dir`) |
| `--audit-log <path>` | Anexa registros JSONL de auditoria (sobrescreve YAML `audit_log`) |
| `--version` | Exibe a versão |

Guias completos: [docs/GUIDE.md](docs/GUIDE.md) · [docs/GUIDE-PT.md](docs/GUIDE-PT.md)

## Configuração

Exemplo mínimo (veja também [`docs/examples/config.yaml`](docs/examples/config.yaml)):

```yaml
platform: auto
transport: ssh
username: "${SWITCH_USER}"
password_command: "pass show lab/access"
enable_password: "${SWITCH_ENABLE}"
default_vlan: "1"
no_data_vlan: "999"
multi_mac_policy: ignore
backup_dir: ./backups
audit_log: ./audit/negev.jsonl
allowed_vlans: ["10", "20", "30"]
protected_vlans: ["999"]
mac_to_vlan:
  "aabbcc": "10"
  "aabbccddeeff": "15"   # prefixo mais longo vence
switches:
  - name: access-a1
    target: 192.0.2.10
    platform: ios
    tags: [access, building-a]
  - name: access-a2
    target: 192.0.2.20
    platform: nxos
    transport: ssh
    tags: [access]
```

Valores de `platform` suportados: `ios`, `nxos`, `dmswitch` (alias legado `dmos`), `dm1200`, `comware`, `auto`.

## Estrutura do Projeto

```
negev/cmd/negev/                 # Ponto de entrada CLI
negev/internal/domain/           # Entidades, ports, serviço de VLAN
negev/internal/application/      # RunTarget, multi-runner, backup
negev/internal/infrastructure/   # Config, transporte, logger de auditoria
negev/internal/platform/         # Drivers: ios, nxos, dmswitch, dm1200, comware
docs/                            # GUIDE + exemplos
bin/                             # Binários compilados (git-ignored)
.make/                           # Scripts de build e instalação
```

## Desenvolvimento

```bash
make build             # Compila o binário em bin/negev
make test              # Executa todos os testes (.gocache local)
make quality           # Formata e executa vet
make install           # Instala o binário em ~/.local/bin
make install-system    # Instala o binário em /usr/local/bin
make uninstall         # Remove de ~/.local/bin
make uninstall-system  # Remove de /usr/local/bin
```

## Licença

Este projeto é licenciado sob a GNU General Public License v3.0 — veja [LICENSE](LICENSE) para detalhes.
