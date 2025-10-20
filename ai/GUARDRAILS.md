# Guardrails Operacionais de IA
Versão 0.1.0 — Atualizado em 2025-10-20

## Licenciamento e Conformidade
- Código licenciado sob MIT; manter cabeçalhos de licença em novos arquivos quando aplicável.
- Respeitar dependências listadas em `core/go.mod`; novas bibliotecas devem ser aprovadas e compatíveis com MIT.

## Segurança e Dados Sensíveis
- Jamais versionar credenciais, arquivos `config.yaml` reais ou saídas completas de switch.
- Utilizar variáveis de ambiente ou cofres para segredos; não deixar dados sensíveis em `ai/STATE.md`.
- Modo sandbox é obrigatório por padrão; somente utilizar `--write` após aprovação humana e registro da janela de mudança.
- Ao lidar com logs, mascarar MACs completos e endereços IP não públicos antes de compartilhar.

## Restrições Técnicas
- Execução prevista para switches Cisco que seguem CLI tradicional; validar compatibilidade antes de ampliar suporte.
- Telnet deve ser usado apenas em redes confiáveis; priorizar SSH quando disponível.
- Não modificar scripts de instalação (`scripts/install.sh`) sem considerar impactos multiplataforma.
- Mantidos limites das VLANs (1-4094); qualquer mudança deve ser refletida na validação em `core/infrastructure/config`.

## Custos e Recursos
- Não há chamadas externas pagas; custos decorrem de tempo de engenharia e janelas de mudança em rede.
- Evitar execuções repetidas em dispositivos de produção durante horários críticos; consolidar ajustes em uma única janela.

## Registro e Auditoria
- Documentar decisões em `ai/DECISIONS.md` e atualizações operacionais em `ai/STATE.md`.
- Guardar evidências de testes (saída resumida de `make test`, validações em sandbox) em artefatos de revisão, não no repositório.

## Resposta a Incidentes
- Caso uma alteração aplique VLAN incorreta, acionar rollback manual conforme runbook da equipe de rede.
- Registrar causa raiz e correção planejada em `ai/PLAN.md` e, quando definitivo, em `ai/DECISIONS.md`.
