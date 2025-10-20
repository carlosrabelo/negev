# Estratégia de Testes
Versão 0.1.0 — Atualizado em 2025-10-20

## Objetivos
- Garantir que regras de VLAN sejam aplicadas corretamente sem causar interrupção de rede.
- Validar parsing e normalização de configurações YAML.
- Permitir evolução segura da camada de transporte (Telnet/SSH) por meio de testes simulados.

## Escopo e Níveis
- **Testes unitários**: foco em `core/infrastructure/config` (validação de VLAN, herança) e `core/domain/services` (decisão de VLAN por MAC).
- **Testes de integração**: simular `SwitchRepository` com mocks que retornam saídas realistas de CLI.
- **Testes manuais guiados**: exercícios em sandbox com switches de laboratório antes de rodar `--write`.

## Ferramentas e Automatização
- Framework padrão `go test`.
- Utilizar `make test` para executar a suíte completa.
- Adotar mocks gerenciados manualmente ou com `testify/mock` (incluir apenas se aprovado e registrado em `core/go.mod`).

## Cobertura Esperada
- Cobertura mínima de 70% nos pacotes `config` e `domain/services`, priorizando fluxos de decisão críticos.
- Toda correção de bug deve vir acompanhada de teste que reproduza o problema.

## Gestão de Dados de Teste
- Armazenar fixtures YAML sintéticos no diretório `core/testdata/` (criar conforme necessidade).
- Remover ou anonimizar MACs/IPs reais antes de adicionar exemplos.

## Integração Contínua
- Ao configurar pipeline CI, incluir etapas `make fmt`, `make lint` e `make test`.
- Publicar saídas resumidas (não logs brutos) para auditoria.
