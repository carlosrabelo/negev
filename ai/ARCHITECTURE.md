# Arquitetura Técnica
Versão 0.1.0 — Atualizado em 2025-10-20

## Visão Geral (C4 Nível 2 Simplificado)
- **Contexto**: operador de rede executa o CLI Negev, que lê um arquivo YAML local e manipula switches (Cisco IOS, Datacom DmOS) via Telnet ou SSH.
- **Objetivo principal**: manter VLANs alinhadas a políticas de prefixos MAC com validação e modo sandbox.

## Contêineres
1. **CLI Negev (`core/cmd/negev`)** — Interface primária; interpreta flags, resolve caminho de configuração e instancia serviços.
2. **Serviços de Aplicação (`core/application/services`)** — Implementam casos de uso (processamento de portas, sincronização de VLAN).
3. **Camada de Domínio (`core/domain`)** — Entidades (`SwitchConfig`, `Device`, `Port`, `AuthPrompt`) e serviços que concentram regras de negócio.
4. **Drivers de Plataforma (`core/platform`)**
   - **Interface `SwitchDriver`** — Define contrato para suporte a diferentes plataformas
   - **Driver IOS (`core/platform/ios`)** — Implementa comandos Cisco IOS (usa `enable`, `show interfaces trunk`)
   - **Driver DmOS (`core/platform/dmos`)** — Implementa comandos Datacom DmOS (sem `enable`, detecta trunks via `Allowed VLANs`)
   - Cada driver define autenticação, comandos de configuração e parsers específicos
5. **Adaptadores de Infraestrutura**
   - **Config Loader (`core/infrastructure/config`)** — Carrega/valida YAML, exige plataforma por switch
   - **Transport (`core/infrastructure/transport`)** — Clientes Telnet/SSH agnósticos à plataforma, executam sequência de autenticação fornecida pelo driver
6. **Switches** — Sistemas externos (Cisco IOS, Datacom DmOS) manipulados via CLI interativa.

## Fluxo Principal
```
Operador → CLI → Config Loader → Serviços de Aplicação
       ↘ flags/verbosidade      ↘ Domain Service → SwitchRepository (Telnet/SSH) → Switch
```
- CLI determina arquivo YAML e resolve target.
- Config loader produz `SwitchConfig` enriquecido com herança global.
- Serviço de VLAN coleta VLANs, portas e tabela MAC via `SwitchRepository`.
- Ajustes aplicados no sandbox; gravação (`write memory`) somente quando `Sandbox` é falso.

## Dependências Externas
- `github.com/ziutek/telnet` para sessão Telnet.
- `golang.org/x/crypto/ssh` (via `golang.org/x/crypto`) para SSH.
- `gopkg.in/yaml.v3` para parsing do arquivo de configuração.

## Considerações de Extensão
- Suporte a novos fabricantes exigirá novo adaptador em `infrastructure/transport`.
- Qualquer alteração na estrutura do YAML deve manter compatibilidade no `Config` e ser refletida em `examples/config.yaml`.
