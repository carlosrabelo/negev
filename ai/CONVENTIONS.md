# Convenções Técnicas
Versão 0.1.0 — Atualizado em 2025-10-20

## Estrutura de Código
- Seguir a divisão hexagonal existente: `domain` (regras), `application` (orquestração), `infrastructure` (adaptações externas) e `cmd` (entrada CLI).
- Novos pacotes devem ser adicionados dentro de `core/`, mantendo nomes curtos e em minúsculas.

## Padrões de Código Go
- Usar `go fmt ./...` e `goimports` (quando disponível) antes de abrir PR.
- Tratar erros com mensagens claras; usar `fmt.Errorf` com contexto em camadas de infraestrutura.
- Evitar logs verbosos por padrão; condicioná-los às flags de verbosidade já existentes.
- Manter comentários apenas quando indispensáveis para explicar regras de rede ou exceções.

## Automação e Build
- Utilize `make build`, `make fmt`, `make lint` e `make test` na raiz para garantir uniformidade.
- Scripts auxiliares (`scripts/*.sh`) devem permanecer compatíveis com ambientes Unix e serem executáveis (`chmod +x`).
- Binaries gerados residem em `bin/` e não devem ser versionados.

## Fluxo de Versionamento
- Confirmar estado limpo com `git status` antes de rodar builds.
- Mensagens de commit em modo imperativo curto em português ou inglês, referenciando contexto (ex.: `Ajusta validação de VLAN`).
- Atualizar `ai/DECISIONS.md` quando uma mudança estrutural for concluída.

## Documentação
- Documentação para usuários finais permanece em `docs/`, enquanto orientações para agentes ficam em `ai/`.
- Sempre que novas flags ou comportamentos forem introduzidos, sincronizar `README-PT.md` e o guia em `docs/GUIDE-PT.md`.
