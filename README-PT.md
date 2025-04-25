# Negev

[![Go Report Card](https://goreportcard.com/badge/github.com/carlosrabelo/negev)](https://goreportcard.com/report/github.com/carlosrabelo/negev)
[![codecov](https://codecov.io/gh/carlosrabelo/negev/branch/master/graph/badge.svg)](https://codecov.io/gh/carlosrabelo/negev)

Ferramenta CLI para automatizar atribuições de VLAN em switches de rede com base em prefixos de endereços MAC.

## Destaques

- Conecta a switches via Telnet ou SSH com detecção automática de plataforma
- Lê tabelas MAC e mapeia prefixos para VLANs a partir de um YAML de configuração
- Atribui VLANs de acesso às portas do switch com base no MAC do dispositivo conectado
- Modo sandbox mostra alterações sem aplicar — use `--write` para executar
- Cria e exclui VLANs para igualar a uma lista permitida com proteção de VLANs
- Suporta Cisco IOS e Datacom DmOS através de um sistema de drivers modular

## Instalação

### Build a partir do código-fonte

```bash
git clone https://github.com/carlosrabelo/negev.git
cd negev
make build
```

Instala em `~/.local/bin`:

```bash
make install
```

## Uso

```bash
negev --target 192.168.1.10                   # sandbox (simular)
negev --target 192.168.1.10 --write           # aplicar alterações
negev --target 192.168.1.10 --create-vlans --write # sincronizar VLANs e aplicar
```

### Flags

| Flag | Descrição |
|------|-----------|
| `--target <ip>` | Endereço IP do switch (obrigatório, deve existir na configuração) |
| `--config <path>` | Caminho do arquivo YAML de configuração (padrão: config.yaml) |
| `--write` | Aplica alterações (sandbox/dry-run por padrão) |
| `--verbose <0-3>` | Nível de verbosidade: 0=nenhum, 1=debug, 2=saída raw, 3=ambos |
| `--create-vlans` | Criar/excluir VLANs para igualar à lista permitida |
| `--version` | Exibe a versão |

## Configuração

Crie `config.yaml` no diretório atual. O Negev também procura em `~/.config/negev/` e `/etc/negev/` (Linux), ou `%APPDATA%\negev\` e `%ProgramData%\negev\` (Windows).

```yaml
platform: auto
transport: telnet
username: admin
password: cisco123
enable_password: cisco123
default_vlan: "1"
no_data_vlan: "999"
allowed_vlans:
  - "10"
  - "20"
  - "30"
protected_vlans:
  - "100"
exclude_macs:
  - "00:11:22:33:44:55"
mac_to_vlan:
  "aabbcc": "10"
  "001122": "20"

switches:
  - target: 192.168.1.10
    platform: ios
  - target: 192.168.1.20
    platform: dmos
    transport: ssh
    exclude_ports:
      - "ethernet 1/1"
```

## Estrutura do Projeto

```
negev/cmd/negev/        # Ponto de entrada CLI
negev/internal/domain/  # Entidades, interfaces e lógica de negócio
negev/internal/application/ # Orquestração de serviços (runner)
negev/internal/infrastructure/ # Carregamento de configuração, transporte (Telnet/SSH), cache de clientes
negev/internal/platform/ # Drivers de plataforma (ios, dmos) e registros
bin/                    # Binário compilado (git-ignored)
.make/                  # Scripts de build e instalação
demos/                  # Arquivos de configuração de exemplo
```

## Desenvolvimento

```bash
make build      # Compila binário em bin/negev
make test       # Executa todos os testes
make quality    # Formata, vet e lint
make install    # Instala em ~/.local/bin
```

## Licença

Este projeto é licenciado sob a Licença MIT — veja [LICENSE](LICENSE) para detalhes.
