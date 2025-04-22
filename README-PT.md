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

## Configuração

Crie `config.yaml` no diretório atual. Negev também procura em `~/.config/negev/` e `/etc/negev/`.

```yaml
platform: auto
transport: telnet
username: admin
password: cisco123
enable_password: cisco123
default_vlan: "1"
no_data_vlan: "999"
mac_to_vlan:
  "aabbcc": "10"

switches:
  - target: 192.168.1.10
    platform: ios
```

## Uso

```bash
negev --target 192.168.1.10          # sandbox (simular)
negev --target 192.168.1.10 --write  # aplicar
negev --target 192.168.1.10 --create-vlans
```

### Flags

| Flag | Descrição |
|------|-----------|
| `--target <ip>` | Endereço IP do switch (obrigatório) |
| `--config <path>` | Caminho do arquivo YAML de configuração |
| `--write` | Aplica alterações (sandbox é o padrão) |
| `--verbose <0-3>` | 0=nenhum, 1=debug, 2=raw, 3=ambos |
| `--create-vlans` | Criar/excluir VLANs para igualar à lista permitida |
| `--version` | Exibe a versão |

## Estrutura do Projeto

```
cmd/negev/              # Ponto de entrada CLI
internal/domain/        # Entidades, interfaces e lógica de negócio
internal/application/   # Orquestração de serviços
internal/infrastructure/ # Carregamento de config, transporte (Telnet/SSH), adapter
internal/platform/      # Drivers de plataforma (ios, dmos)
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
