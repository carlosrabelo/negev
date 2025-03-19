# Guia do Usuário Negev

O Negev é uma ferramenta CLI para automatizar atribuições de VLAN em switches de acesso de rede com base em prefixos de endereços MAC. Ele se conecta aos switches via Telnet ou SSH, lê a tabela de endereços MAC, mapeia os dispositivos conectados às suas respectivas VLANs de destino e configura as portas do switch adequadamente.

## Instalação

Para compilar e instalar o binário a partir do código-fonte, execute:

```bash
make install
```

Isso instala o binário em `~/.local/bin/negev` (Linux) ou `/usr/local/bin/negev`.

## Configuração

O Negev utiliza um arquivo de configuração YAML. Por padrão, ele procura por `config.yaml` no diretório atual, mas também recorre aos seguintes caminhos do sistema:

- `./config.yaml`
- `~/.config/negev/config.yaml` (Linux)
- `/etc/negev/config.yaml` (Linux)
- `%APPDATA%\negev\config.yaml` (Windows)
- `%ProgramData%\negev\config.yaml` (Windows)

### Esquema de Configuração

A estrutura de configuração suporta definições globais que podem ser sobrescritas individualmente para cada switch.

#### Configurações Globais

Estas configurações se aplicam a todos os switches, a menos que sejam sobrescritas:

```yaml
# Suportado: auto, ios, dmos. "auto" executa 'show version' para detectar a plataforma.
platform: auto

# Suportado: telnet, ssh
transport: telnet

# Credenciais globais de autenticação
username: admin
password: cisco123
enable_password: cisco123

# VLAN padrão para portas ativas com endereços MAC não reconhecidos
default_vlan: "1"

# VLAN de quarentena para portas onde nenhum endereço MAC é detectado
no_data_vlan: "999"

# Lista global de VLANs permitidas que o Negev tem permissão para criar ou modificar
allowed_vlans:
  - "10"
  - "20"
  - "30"

# Lista global de VLANs protegidas que nunca devem ser excluídas
protected_vlans:
  - "100"

# Endereços MAC globais a serem ignorados (correspondências exatas, normalizados)
exclude_macs:
  - "00:11:22:33:44:55"

# Mapeamento global de prefixos MAC (primeiros 6 dígitos hexadecimais) para IDs de VLAN
mac_to_vlan:
  "aabbcc": "10"
  "001122": "20"
```

#### Configurações por Switch e Sobrescritas

Você pode definir switches individuais no bloco `switches`. Cada switch pode sobrescrever credenciais, transporte, VLANs e mapeamentos de MAC para VLAN:

```yaml
switches:
  - target: 192.168.1.10
    platform: ios
    transport: telnet
    mac_to_vlan:
      "001122": "30" # Sobrescreve o mapeamento global para o prefixo 001122

  - target: 192.168.1.20
    platform: dmos
    transport: ssh
    username: operator
    password: senha_segura
    # Desativa a herança de um mapeamento de prefixo MAC global definindo-o como "0", "00" ou ""
    mac_to_vlan:
      "aabbcc": "0" 
    exclude_ports:
      - "ethernet 1/1"
      - "ethernet 1/2"
```

### Regras de Mesclagem de Configuração

1. **Mapa MacToVlan**: Os mapeamentos de prefixo globais são mesclados com os mapeamentos específicos de cada switch. Mapeamentos do switch sobrescrevem os globais para o mesmo prefixo. Se um mapeamento do switch definir a VLAN de um prefixo como `"0"`, `"00"` ou `""`, esse mapeamento é removido inteiramente para aquele switch.
2. **Lista ExcludeMacs**: Os MACs excluídos globais e do switch são mesclados, normalizados e duplicatas são removidas.
3. **Lista ExcludePorts**: Definida apenas no nível do switch. Portas nesta lista são completamente ignoradas durante a atribuição de VLAN.
4. **AllowedVlans e ProtectedVlans**: As listas específicas de cada switch são mescladas com as listas globais e duplicatas são removidas.

---

## Uso

```bash
negev --target <ip_do_switch> [flags]
```

### Flags

| Flag | Descrição |
|---|---|
| `--target <ip>` | Endereço IP do switch para se conectar (obrigatório, deve existir na configuração) |
| `--config <path>` | Caminho do arquivo de configuração YAML |
| `--write` | Aplica as alterações no switch (o modo sandbox/dry-run está ativo por padrão) |
| `--verbose <0-3>` | Nível de verbosidade: `0` = nenhum, `1` = logs de debug, `2` = comunicação de rede raw com o switch, `3` = ambos |
| `--create-vlans` | Cria automaticamente VLANs permitidas ausentes e exclui as não autorizadas (requer `--write` para aplicar) |
| `--version` | Exibe a versão e hora da compilação |

---

## Funcionamento

1. **Conexão e Resolução do Driver**: O Negev se conecta ao switch usando o transporte configurado (Telnet ou SSH). Se a plataforma estiver definida como `auto`, ele executa o comando `show version` e detecta se é um switch Cisco IOS ou Datacom DmOS.
2. **Segurança e Cache de Clientes**: 
   - **Telnet**: Transmite credenciais em texto claro (gera um aviso no log).
   - **SSH**: Utiliza emulação de PTY VT100 com eco suprimido. Desativa a validação do host-key (gera um aviso no log).
   - **Cache**: Os switches reutilizam conexões de rede do cache caso compartilhem as mesmas credenciais, transporte e destino.
3. **Coleta de Informações**: O Negev consulta o switch sobre VLANs ativas, portas de trunk, status das interfaces ativas e a tabela dinâmica de endereços MAC.
   - **Cisco IOS**: Executa `show vlan brief`, `show interfaces trunk`, `show interfaces status` e `show mac address-table dynamic`.
   - **Datacom DmOS**: Executa `show vlan table`, `show interfaces switchport` (caxeado por execução para evitar chamadas duplicadas), `show interfaces status` e `show mac-address-table`.
4. **Exclusão de Portas Trunk**: Interfaces detectadas como portas trunk são ignoradas automaticamente para evitar interrupções de rede.
5. **Lógica de Atribuição de VLAN**:
   - Para cada porta de acesso, o Negev inspeciona o endereço MAC conectado.
   - Se múltiplos endereços MAC forem detectados na mesma porta, um aviso de segurança é registrado e a porta é pulada.
   - O endereço MAC é normalizado (removendo `:` e `.`, convertendo para minúsculas) e seu prefixo de 6 caracteres é verificado no mapa `mac_to_vlan`.
   - Se um prefixo correspondente for encontrado, a VLAN de destino é atribuída.
   - Se nenhum prefixo corresponder, a porta é atribuída à `default_vlan`.
   - Se nenhum endereço MAC estiver ativo na porta, a porta é atribuída à `no_data_vlan`.
   - Se a VLAN de destino não existir no switch, a atribuição é ignorada com uma mensagem de erro.
6. **Sincronização de VLAN (`--create-vlans`)**:
   - Compara as VLANs ativas do switch com a lista `allowed_vlans`.
   - Cria qualquer VLAN definida em `allowed_vlans` que esteja ausente no switch.
   - Exclui qualquer VLAN presente no switch que *não* esteja em `allowed_vlans` e *not* seja protegida.
7. **Execução ou Simulação**: Se `--write` for omitido, o Negev exibe exatamente os comandos que enviaria. Se `--write` for especificado, os comandos são executados e a configuração é salva (`write memory` no IOS, `copy running-config startup-config` no DmOS).

---

## Modo Sandbox

Por padrão, o Negev executa em um modo sandbox seguro, permitindo visualizar as alterações do switch antes de aplicá-las:

```bash
# Visualizar alterações em um Cisco IOS
negev --target 192.168.1.10

# Aplicar alterações em um Datacom DmOS via SSH
negev --target 192.168.1.20 --write
```

---

## Normalização de Endereço MAC

Endereços MAC são normalizados removendo os separadores (`:`, `.`) e convertendo os caracteres para minúsculas:

- `00:11:22:AA:BB:CC` → `001122aabbcc`
- `0011.22aa.bbcc` → `001122aabbcc`

A chave de busca no mapeamento `mac_to_vlan` são os primeiros 6 caracteres do MAC normalizado (ex: `001122`).

---

## Proteção de VLAN

VLANs são protegidas contra exclusão (ao executar com `--create-vlans`) sob duas condições:
- **Auto-protection**: VLANs no intervalo de `1000` a `4094` são protegidas automaticamente.
- **Proteção Explícita**: VLANs listadas em `protected_vlans` no arquivo de configuração.

VLANs protegidas nunca são excluídas pelo Negev.
