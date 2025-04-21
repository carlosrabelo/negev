# Guia do Usuário Negev

Negev é uma ferramenta CLI para automatizar atribuições de VLAN em switches de rede com base em prefixos de endereços MAC.

## Instalação

```bash
make install
```

Instala o binário em `~/.local/bin/negev` (Linux) ou `/usr/local/bin/negev`.

## Configuração

Crie um arquivo `config.yaml`. O Negev procura em:

- `./config.yaml`
- `~/.config/negev/config.yaml` (Linux)
- `/etc/negev/config.yaml` (Linux)
- `%APPDATA%\negev\config.yaml` (Windows)
- `%ProgramData%\negev\config.yaml` (Windows)

### Configurações Globais

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
```

### Sobrescritas por Switch

```yaml
switches:
  - target: 192.168.1.10
    platform: ios
    mac_to_vlan:
      "001122": "30"
```

## Uso

```bash
negev --target 192.168.1.10
```

### Flags

| Flag | Descrição |
|---|---|
| `--target <ip>` | Endereço IP do switch (obrigatório) |
| `--config <path>` | Caminho do arquivo YAML de configuração |
| `--write` | Aplica as alterações (sandbox é o padrão) |
| `--verbose <0-3>` | 0=nenhum, 1=debug, 2=raw, 3=ambos |
| `--create-vlans` | Criar/excluir VLANs para igualar à lista permitida |
| `--version` | Exibe a versão |

## Funcionamento

1. Conecta ao switch via Telnet ou SSH
2. Detecta a plataforma (Cisco IOS) automaticamente ou usa o driver configurado
3. Lê a tabela MAC e as portas ativas
4. Mapeia prefixos MAC para VLANs usando a configuração
5. Configura VLANs de acesso nas portas do switch
6. Exibe alterações simuladas no modo sandbox (use `--write` para aplicar)

## Modo Sandbox

Por padrão, o Negev executa em modo sandbox mostrando o que seria alterado sem aplicar nada. Use `--write` para aplicar.

```bash
negev --target 192.168.1.10          # sandbox (simular)
negev --target 192.168.1.10 --write  # aplicar
```

## Formato de MAC

Endereços MAC são normalizados para 12 caracteres hex minúsculos:

- `aa:bb:cc:dd:ee:ff` → `aabbccddeeff`
- `aabb.ccdd.eeff` → `aabbccddeeff`

O mapeamento de VLAN usa os primeiros 6 caracteres (prefixo) como chave.

## Proteção de VLAN

- VLANs 1000–4094 são automaticamente protegidas
- VLANs adicionais podem ser listadas em `protected_vlans`
- VLANs protegidas nunca são excluídas, mesmo com `--create-vlans`
