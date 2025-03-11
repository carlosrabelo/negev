# Negev

Automatiza atribuições de VLAN em switches de acesso a partir de prefixos de endereços MAC.


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
switches:
  - target: 192.168.1.10
    platform: ios
```

## Desenvolvimento

```bash
make build             # Compila o binário em bin/negev
make test              # Executa todos os testes
make quality           # Formata, vet e lint
make install           # Instala o binário em ~/.local/bin
make install-system    # Instala o binário em /usr/local/bin
make uninstall         # Remove de ~/.local/bin
make uninstall-system  # Remove de /usr/local/bin
```

## Licença

Este projeto é licenciado sob a GNU General Public License v3.0 — veja [LICENSE](LICENSE) para detalhes.
