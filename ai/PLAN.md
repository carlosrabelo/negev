# Plano de Prioridades
Versão 0.1.0 — Atualizado em 2025-10-20

## Horizonte Imediato (P0)
1. Criar suíte mínima de testes para `core/infrastructure/config` cobrindo validação de VLAN e mescla de parâmetros.
2. Implementar mocks de `SwitchRepository` para testar `core/domain/services/VLANServiceImpl` sem dispositivos reais.
3. Revisar scripts de build (`scripts/*.sh`) para garantir compatibilidade com Go 1.24 e registrar evidências em `ai/DECISIONS.md`.

## Curto Prazo (P1)
- Automatizar lint (`golangci-lint`) em pipeline de CI.
- Documentar fluxo de rollback operacional no `docs/GUIDE-PT.md`.

## Monitoramento
- Atualizar `ai/STATE.md` a cada avanço significativo ou bloqueio.
