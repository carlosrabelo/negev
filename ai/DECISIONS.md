# Registro de Decisões Técnicas
Versão 0.1.0 — Atualizado em 2025-10-20

## 2025-10-20 — Arquitetura Hexagonal em Go
- **Contexto:** Necessidade de separar lógica de negócio da infraestrutura de acesso ao switch.
- **Decisão:** Manter módulos `domain`, `application`, `infrastructure` e `cmd` dentro de `core/`, preservando interfaces (`SwitchRepository`) para desacoplamento.
- **Consequências:** Facilita testes com mocks de transporte e substituição futura de implementações Telnet/SSH.

## 2025-10-20 — Configuração central via YAML
- **Contexto:** Operadores precisam administrar múltiplos switches com parâmetros globais e específicos.
- **Decisão:** Utilizar arquivo YAML com herança de valores globais e normalização de MACs em `core/infrastructure/config`.
- **Consequências:** Simplifica manutenção de playbooks, exige validação rigorosa de VLANs (1-4094) e proteção contra duplicidades.

## 2025-10-20 — Execução em Sandbox por padrão
- **Contexto:** Alterações de VLAN são sensíveis e podem causar indisponibilidade.
- **Decisão:** Executar com `Sandbox = true` por padrão, exigindo `--write` para aplicar alterações e registrando persistência apenas quando necessário.
- **Consequências:** Minimiza riscos em produção, demanda testes completos no modo simulado antes de qualquer mudança real.

## 2025-10-20 — Arquitetura de Drivers para Plataformas
- **Contexto:** Necessidade de suportar múltiplas plataformas de switches (Cisco IOS, Datacom DmOS) com comandos e comportamentos diferentes.
- **Decisão:**
  - Criar interface `SwitchDriver` com métodos para comandos específicos de cada plataforma
  - Cada driver implementa autenticação, parsing de comandos e detecção de plataforma
  - Plataforma é obrigatória por switch (sem fallback global)
  - Drivers podem usar cache interno para otimizar comandos lentos
- **Consequências:**
  - Facilita adição de novas plataformas sem modificar código existente
  - Permite otimizações específicas por plataforma (ex: cache de `show interfaces switchport` no DmOS)
  - Requer manutenção de parsers específicos para cada plataforma

## 2025-10-20 — Autenticação por Driver
- **Contexto:** Diferentes plataformas têm sequências de autenticação diferentes (IOS usa `enable`, DmOS não)
- **Decisão:** Cada driver define sua própria sequência de autenticação via `GetAuthenticationSequence()`
- **Consequências:**
  - TelnetClient/SSHClient são agnósticos à plataforma
  - Facilita suporte a novas plataformas com processos de login diferentes
  - Autenticação é configurada antes da conexão baseada no driver selecionado

## 2025-10-20 — Detecção de Trunks no DmOS
- **Contexto:** DmOS não tem comando explícito para listar trunks como IOS (`show interfaces trunk`)
- **Decisão:** Detectar trunks analisando `Allowed VLANs` no output de `show interfaces switchport` - portas com VLANs tagged `(s,t)` são consideradas trunk
- **Consequências:**
  - Parser específico para DmOS mais complexo
  - Requer cache do comando `show interfaces switchport` para evitar execução duplicada
  - Detecção baseada em comportamento observado, não em flag explícita

## 2025-10-20 — Timeouts Aumentados para DmOS
- **Contexto:** Comandos do DmOS como `show interfaces switchport` e `show mac-address-table` são muito lentos (>60s)
- **Decisão:** Aumentar timeout global de 30s para 120s em Telnet e SSH
- **Consequências:**
  - Comandos lentos podem completar sem timeout
  - Possível espera longa em caso de problemas de rede
  - Necessário monitorar performance em produção
