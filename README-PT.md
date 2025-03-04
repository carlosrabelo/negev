# Negev

Automatiza atribuições de VLAN em switches de acesso a partir de prefixos de endereços MAC.


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
