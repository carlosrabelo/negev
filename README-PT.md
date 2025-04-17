# Negev

CLI que automatiza atribuições de VLAN em switches de acesso a partir de prefixos MAC.
Conecta via Telnet ou SSH, lê a tabela MAC e configura portas de acesso com política YAML.
A arquitetura segue limites hexagonais / DDD
(`domain` → `application` → `infrastructure` / drivers `platform`).

English: [README.md](README.md).

## Destaques

- Drivers multi-vendor: Cisco IOS, Cisco NX-OS, Datacom DmSwitch 3000, Datacom DM1200E, 3Com/H3C Comware (detecção `auto`)
- Seguro por padrão: sandbox sem `--write`; VLANs protegidas (incluindo 1000–4094)
- Transporte dual: SSH (preferencial) e Telnet com cache de clientes
- Segredos fora do YAML: expansão `${ENV}`, `password_command` e `--ask-pass`
- Execução multi-switch: nomes, tags, `--all` / `--group`, `--concurrency` limitado
- Matching `mac_to_vlan` por prefixo mais longo, `multi_mac_policy`, backups e auditoria JSONL

## Pré-requisitos

- **Go 1.22.3+** — necessário para compilar a partir do código-fonte; [download](https://go.dev/dl/)

## Instalação

```bash
git clone https://github.com/carlosrabelo/negev.git
cd negev
make build
make install              # ~/.local/bin
# make install-system     # /usr/local/bin
```

Ou:

```bash
go install github.com/carlosrabelo/negev/negev/cmd/negev@latest
```

Documentação completa: [docs/README-PT.md](docs/README-PT.md).

## Uso

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml
negev --target 192.0.2.10 --config docs/examples/config.yaml --write
negev --all --config docs/examples/config.yaml --concurrency 4
negev --group access --config docs/examples/config.yaml
```

Códigos de saída: `0` sucesso / sem mudanças pendentes no sandbox, `1` erro, `2` mudanças simuladas no sandbox.

Flags: [docs/cli-pt/](docs/cli-pt/) · English: [docs/cli/](docs/cli/) · guia: [docs/GUIDE-PT.md](docs/GUIDE-PT.md)

## Configuração

Snippet mínimo (esquema completo no [guia](docs/GUIDE-PT.md); amostra carregável:
[`docs/examples/config.yaml`](docs/examples/config.yaml)):

```yaml
platform: auto
transport: ssh
username: "${SWITCH_USER}"
password_command: "pass show lab/access"
default_vlan: "1"
mac_to_vlan:
  "aabbcc": "10"
switches:
  - name: access-a1
    target: 192.0.2.10
    platform: ios
    tags: [access]
```

## Estrutura do Projeto

```
negev/cmd/negev/                 # Ponto de entrada CLI
negev/internal/                  # domain, application, infrastructure, drivers
docs/                            # GUIDE, cli/, cli-pt/, examples/
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
