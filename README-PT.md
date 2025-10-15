# Negev

[![Go Report Card](https://goreportcard.com/badge/github.com/carlosrabelo/negev)](https://goreportcard.com/report/github.com/carlosrabelo/negev)

**Negev** é uma ferramenta de automação de VLAN para switches Cisco via Telnet ou SSH. Ele atribui dinamicamente VLANs com base em prefixos de endereços MAC, gerencia configurações de switch e mantém seu playbook sincronizado com o que está conectado em cada interface.

## Recursos

- **Gerenciamento Telnet**: Conecta-se a switches Cisco via Telnet para recuperar tabelas de endereços MAC e configurar VLANs.
- **Gerenciamento SSH**: Conecta-se a switches Cisco via SSH quando Telnet está desabilitado ou não é desejado.
- **Atribuição de VLAN Baseada em MAC**: Atribui VLANs com base nos primeiros três bytes de endereços MAC, com uma VLAN padrão para dispositivos não mapeados.
- **Modo Sandbox**: Simula alterações de configuração sem aplicá-las ao switch.
- **Persistência de Configuração**: Salva alterações na configuração em execução do switch (com flag `-w`).
- **Exclusão de MAC**: Ignora endereços MAC especificados durante a atribuição de VLAN.
- **Exclusão de Portas**: Permite pular interfaces que nunca devem ser tocadas.
- **Detecção de Interface Trunk**: Ignora automaticamente interfaces trunk para evitar configuração incorreta.
- **Criação de VLAN**: Opcionalmente cria VLANs ausentes no switch (com flag `-c`).
- **Logging Detalhado**: Fornece saída de debug detalhada para solução de problemas (use `-v 1`).
- **Exibição de Saída Bruta**: Mostra saídas brutas do switch para debug (use `-v 2` ou `-v 3`).
- **Validação de VLAN**: Opcionalmente pula verificações de existência de VLAN (com flag `-s`).

## Manuais do Usuário

- [User Guide (English)](docs/GUIDE.md)
- [Guia do Usuário (Português)](docs/GUIDE-PT.md)

## Instalação

Clone o repositório e construa a ferramenta usando os seguintes comandos:

```bash
git clone https://github.com/carlosrabelo/negev.git
cd negev
go build -o negev ./core/cmd/negev
```

Ou use os helpers do Makefile (recomendado):

```bash
make build
./bin/negev -t 192.168.1.1
```

## Configuração

A configuração é definida em um arquivo YAML, especificando a VLAN padrão, mapeamentos MAC-to-VLAN e exclusões. Um exemplo completo está em `examples/config.yaml`. Abaixo está um trecho:

```yaml
transport: "telnet"
username: "admin"
password: "password"
enable_password: "enable_password"
default_vlan: "10"
no_data_vlan: "99"
exclude_macs:
  - "d8:d3:85:d7:0d:b7"
  - "ac:16:2d:34:bb:da"
mac_to_vlan:
  "3c:2a:f4": "30"  # Brother
  "dc:c2:c9": "30"  # Canon
  "00:c8:8b": "50"  # Cisco AP
switches:
  - target: "192.168.1.1"
    transport: "ssh"
    username: "admin"
    password: "password"
    enable_password: "enable_password"
    default_vlan: "10"
    no_data_vlan: "99"
    exclude_macs:
      - "00:11:22:33:44:55"
    exclude_ports:
      - "gi1/0/24"
    mac_to_vlan:
      "a4:bb:6d": "20"  # Custom device
```

#### Campos Globais Obrigatórios:

- **transport (opcional)** Transporte global para sessões de switch (`telnet` por padrão, aceita `ssh`).
- **username, password, enable_password** Credenciais padrão para switches (usadas se não especificadas por switch).
- **default_vlan** VLAN padrão global para MACs não mapeados.
- **no_data_vlan** VLAN de quarentena global para dispositivos desconectados.
- **exclude_macs (opcional)** Lista de endereços MAC completos para ignorar.
- **mac_to_vlan (opcional)** Mapeamento de prefixos MAC (primeiros 3 bytes) para VLANs.

#### Campos por Switch:

- **target** Endereço IP do switch Cisco.
- **transport (opcional)** Sobrescreve o transporte global (`telnet` ou `ssh`).
- **username, password, enable_password (opcional)** Credenciais específicas do switch (volta para o global).
- **default_vlan (opcional)** VLAN padrão específica do switch (volta para o global).
- **no_data_vlan (opcional)** VLAN de quarentena específica do switch (volta para o global).
- **exclude_macs (opcional)** MACs específicos do switch para ignorar (mesclado com global).
- **exclude_ports (opcional)** Lista de interfaces para pular (comparação não diferencia maiúsculas/minúsculas).
- **mac_to_vlan (opcional)** Mapeamentos MAC-to-VLAN específicos do switch (mesclado com global).

## ⚠️ Segurança

- **Telnet** Telnet é inseguro e transmite credenciais em texto plano. Use apenas em redes confiáveis.
- **Modo Sandbox** Sempre teste em modo sandbox (padrão) antes de aplicar alterações com -w.
- **Credenciais** Armazene informações sensíveis (username, password, enable_password) com segurança.

## Limitações

- **Transporte** Telnet é o padrão; suporte SSH depende do dispositivo ter uma CLI interativa similar ao Telnet.
- **Switch Único** Cada execução processa um switch (especificado com `-t`).
- **Sem Reversão** Alterações não são revertidas automaticamente em caso de falha.
- **MAC Único por Porta** Portas com múltiplos endereços MAC são ignoradas para evitar ambiguidade.
- **Parse de Saída do Switch** A ferramenta assume formatos de saída padrão de switches Cisco; formatos inesperados podem causar erros de parse.

## Estrutura do Projeto

- `core/cmd/negev`: Ponto de entrada da CLI e tratamento de flags
- `core/infrastructure/config`: Parse YAML e validação de configuração
- `core/infrastructure/transport`: Clientes de transporte Telnet/SSH com cache
- `core/application/services`: Serviços de aplicação para VLAN
- `core/domain/`: Entidades de domínio e lógica de negócio
- `docs/`: Guias do usuário em Inglês (`GUIDE.md`) e Português (`GUIDE-PT.md`)
- `examples/`: Arquivos de configuração de referência como `config.yaml`
- `bin/`: Artefatos de build gerados por `make build`
- `scripts/`: Scripts de build e utilitários

## Contribuindo

Contribuições são bem-vindas! Por favor, envie issues ou pull requests para o repositório GitHub.