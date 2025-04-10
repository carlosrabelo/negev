# Negev - Cisco VLAN Manager

Negev é uma ferramenta escrita em Go para automatizar a configuração de VLANs em portas de switches Cisco com base em endereços MAC. Ele utiliza uma conexão Telnet e um arquivo de configuração YAML para definir as regras de mapeamento de MAC para VLANs.

## Funcionalidades

- **Conexão Telnet**: Conecta-se a switches Cisco para gerenciar configurações.
- **Modo Sandbox**: Simula alterações sem aplicá-las (padrão; desative com `-x`).
- **Configuração YAML**: Define host, credenciais, VLAN padrão e mapeamentos MAC-to-VLAN.
- **Otimização**: Acumula comandos e executa `write memory` apenas uma vez no final, se houver alterações.
- **Depuração**: Logs detalhados ativados com `-d`.
- **Timeout**: 30 segundos para operações Telnet, evitando travamentos.
