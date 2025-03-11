# Guia do Usuário Negev

O Negev automatiza a atribuição de VLANs em switches de acesso Cisco IOS a partir de prefixos MAC em um arquivo YAML.

## Conexão

Use Telnet ou SSH. Telnet envia credenciais em texto claro. A verificação de host key SSH está desabilitada — use apenas em redes confiáveis.

```bash
negev --target 192.168.1.10
negev --target 192.168.1.10 --write
```

## Configuração

Veja `demos/config.yaml`. Plataformas: `ios`, `dmos` e `auto`.
