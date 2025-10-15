# Manual do Usuário Negev (Português)

Este manual mostra como operar o **Negev** depois de ter o binário disponível. O foco é explicar o fluxo de uso diário, a configuração e a solução de problemas.

## 1. Visão Geral
- Negev automatiza a atribuição de VLAN em switches Cisco usando Telnet ou SSH.
- As decisões se baseiam em prefixos de MAC, exclusões explícitas e VLAN padrão.
- Por padrão, Negev roda em *modo sandbox* (apenas simula). Use `--write` para aplicar mudanças de verdade.

## 2. Arquivos Necessários
- **Binário**: `negev` (baixado do release do GitHub ou outra distribuição sua).
- **Configuração**: arquivo YAML com switches, credenciais, regras de VLAN e exclusões. Há um exemplo em `examples/config.yaml`.

Coloque `config.yaml` ao lado do binário ou em um destes lugares:
- Diretório atual (`./config.yaml`).
- Linux: `~/.config/negev/config.yaml` ou `/etc/negev/config.yaml`.
- Windows: `%APPDATA%\negev\config.yaml` ou `%ProgramData%\negev\config.yaml`.

## 3. Entendendo o Arquivo de Configuração
Cada seção controla o comportamento do Negev.

### 3.1 Configurações Globais
```yaml
transport: "ssh"        # protocolo padrão (telnet ou ssh)
username: "admin"       # usuário padrão
password: "secret"       # senha padrão
enable_password: "enable" # senha para modo enable
default_vlan: "10"       # VLAN usada quando não há mapeamento
no_data_vlan: "99"       # VLAN aplicada quando a porta fica sem dados
exclude_macs:
  - "00:11:22:33:44:55"  # MACs ignorados completamente
mac_to_vlan:
  "aa:bb:cc": "20"      # prefixo → VLAN
```

- Credenciais globais servem como fallback para switches que não definem credenciais próprias.
- `mac_to_vlan` usa os três primeiros bytes do MAC (seis caracteres hexadecimais) para decidir a VLAN.
- `exclude_macs` lista MACs completos que devem ser ignorados.

### 3.2 Ajustes por Switch
```yaml
switches:
  - target: "192.168.1.10"
    transport: "telnet"
    username: "switch-admin"
    password: "switch-pass"
    enable_password: "switch-enable"
    default_vlan: "30"
    no_data_vlan: "88"
    exclude_macs:
      - "d8:d3:85:d7:0d:b7"
    exclude_ports:
      - "Gi1/0/24"
    mac_to_vlan:
      "dc:c2:c9": "50"
      "00:c8:8b": "70"
```

- `target` é obrigatório e deve bater com o `--target` na hora de rodar.
- `transport`, credenciais e VLANs definidas aqui substituem os valores globais só para esse switch.
- `exclude_ports` (não diferencia maiúsculas/minúsculas) evita que o Negev toque em portas sensíveis.
- Ao mesclar, as regras do switch têm prioridade sobre as globais para o mesmo prefixo.

### 3.3 Boas Práticas para o YAML
- Use letras minúsculas para prefixos e MACs completos.
- Respeite indentação de dois espaços (nada de tabs).
- Deixe comentários explicando mapeamentos ou exclusões fora do comum.
- Se for compartilhar o arquivo, remova senhas ou use placeholders ligados a variáveis de ambiente.

## 4. Rodando o Negev
Comando básico:
```bash
negev --target 192.168.1.10
```

### Flags Úteis
- `--config caminho/arquivo.yaml` usa outro arquivo de configuração.
- `--write` aplica mudanças (sem essa flag o Negev apenas simula).
- `--verbose 1` mostra logs de debug (decisões de merge, exclusões).
- `--verbose 2` exibe saída crua do switch.
- `--verbose 3` junta debug e saída crua.
- `--create-vlans` cria VLANs exigidas pelas regras quando não existem no switch.

### Fluxo Sugerido
1. Rode primeiro em modo sandbox para revisar:
   ```bash
   negev --target 192.168.1.10 --verbose 1
   ```
2. Se o relatório estiver correto, execute com aplicação real:
   ```bash
   negev --target 192.168.1.10 --write --verbose 1
   ```

### Interpretando a Saída
- Linhas com `SANDBOX` mostram os comandos que seriam executados.
- `Configured Gi1/0/1 to VLAN 20` confirma uma mudança bem sucedida.
- `Warning: Multiple MACs...` indica que a porta foi ignorada para evitar risco.
- `Error: VLAN 50 does not exist` mostra que a VLAN precisa ser criada (use `--create-vlans` se fizer sentido).

## 5. Mantendo a Configuração
- Versione o YAML sem dados sensíveis.
- Atualize `mac_to_vlan` quando surgirem novos tipos de dispositivos.
- Revise `exclude_macs` e `exclude_ports` periodicamente para evitar lixo.
- Compartilhe pedaços de configuração com a equipe para padronizar os switches.

## 6. Resolução de Problemas
| Sintoma | Possível Causa | Ação sugerida |
| --- | --- | --- |
| "target ... not registered" | Switch não está na lista | Verifique se o IP está em `switches` com a grafia correta |
| "No devices found" | Switch não retornou tabela MAC | Confirme se o comando existe no modelo; tente `--verbose 2` para ver a resposta |
| Erros de SSH | Credenciais erradas ou SSH bloqueado | Revise usuário/senha, habilite SSH ou use Telnet temporariamente |
| VLAN repetindo | Switch reverte ou há múltiplos MACs | Verifique trunks, porta com múltiplos MACs ou políticas de segurança |

## 7. Perguntas Frequentes
- **Posso rodar em vários switches ao mesmo tempo?** Não. Execute o Negev uma vez por switch com o `--target` correto.
- **Dá para simular mesmo com `--write`?** Não. Sem `--write`, sempre sandbox. Para aplicar execute novamente com `--write`.
- **Negev mexe em portas trunk?** Não. Ele detecta trunk e ignora.
- **E portas com mais de um MAC?** São ignoradas para não arriscar configurações erradas.

## 8. Checklist Antes de Aplicar
- As VLANs necessárias já existem? Caso contrário, pense em usar `--create-vlans`.
- Portas críticas estão em `exclude_ports`?
- Conferiu os prefixos (`aa:bb:c1` vs `aa:bb:c1`)?
- Em ambientes sensíveis, rode com `--verbose 3` na primeira execução real.

Seguindo esse manual você ganha mais confiança no uso do Negev, ajustando VLANs com menos esforço e risco.
