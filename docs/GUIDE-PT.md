# Guia do Usuário Negev

O Negev é uma CLI para automatizar atribuições de VLAN em switches de acesso a partir
de prefixos MAC. Conecta via Telnet ou SSH, lê a tabela MAC, resolve VLANs-alvo pela
política YAML e configura portas de acesso. A organização hexagonal / DDD mantém a
lógica de domínio livre de I/O de socket; drivers de plataforma e adaptadores de
transporte ficam nas bordas.

O inglês é a fonte da verdade técnica: [GUIDE.md](GUIDE.md).

## Arquitetura

| Camada | Caminho | Papel |
|---|---|---|
| CLI | `negev/cmd/negev` | Flags, códigos de saída, `--ask-pass` |
| Application | `negev/internal/application` | `RunTarget`, multi-runner, orquestração de backup |
| Domain | `negev/internal/domain` | Entidades, ports, serviço VLAN plan-then-apply |
| Infrastructure | `negev/internal/infrastructure` | Loader YAML, Telnet/SSH, auditoria JSONL |
| Platform | `negev/internal/platform` | Registry + IOS / NX-OS / DmSwitch / DM1200 / Comware |

Fluxo por alvo: carregar/mesclar config → resolver driver (`auto` usa `show version` /
equivalente) → backup pré-write opcional → `ProcessPorts()` → save opcional → append
de auditoria por mudança de porta.

Clientes de transporte são cacheados por SHA-256 de `{Transport, Target, Username, Password, EnablePassword}`.
Telnet envia credenciais em claro (avisa). SSH usa PTY e ignora verificação de host key
(avisa)—use apenas em redes confiáveis.

## Instalação

```bash
make install          # ~/.local/bin/negev
make install-system   # /usr/local/bin/negev
```

Ou `go install github.com/carlosrabelo/negev/negev/cmd/negev@latest`.

## Configuração

Ordem de busca quando `--config` é omitido e o nome é `config.yaml`:

- `./config.yaml`
- `~/.config/negev/config.yaml` (Linux/macOS via diretório de config do usuário)
- `/etc/negev/config.yaml` (Linux)
- `%APPDATA%\negev\config.yaml` / `%ProgramData%\negev\config.yaml` (Windows)

Sobrescreva a raiz de busca em testes com `XDG_CONFIG_HOME` / `HOME` / `APPDATA` conforme necessário.
Os campos YAML opcionais `backup_dir` e `audit_log` são globais.

### Esquema YAML (global + switch)

```yaml
platform: auto                 # ios | nxos | dmswitch | dm1200 | comware | auto
                               # alias legado: dmos → dmswitch
transport: ssh                 # telnet | ssh
username: "${SWITCH_USER}"     # ${VAR} expandido no Load; $${VAR} → literal ${VAR}
password: ""                   # opcional se password_command / --ask-pass
password_command: "pass show lab/sw"
enable_password: ""
enable_password_command: "pass show lab/enable"
default_vlan: "1"              # obrigatório
no_data_vlan: "999"            # quarentena opcional para portas up sem MAC
multi_mac_policy: ignore       # ignore | alert | priority
backup_dir: ./backups
audit_log: ./audit/negev.jsonl
allowed_vlans: ["10", "20", "30"]
protected_vlans: ["999"]
exclude_macs: ["00:11:22:33:44:55"]
mac_to_vlan:
  "aabbcc": "10"               # prefixos hex de comprimento par, 2..12
  "aabbccddeeff": "15"         # match mais longo vence

switches:
  - name: access-a1            # opcional, único
    target: 192.0.2.10         # obrigatório
    tags: [access, building-a]
    platform: ios
    transport: ssh
    # sobrescritas por switch: credenciais, VLANs, mac_to_vlan, exclude_*, multi_mac_policy
    exclude_ports: ["Gi1/0/24"]
    mac_to_vlan:
      "aabbcc": "0"            # remove prefixo herdado neste switch
```

### Regras de mescla

1. **mac_to_vlan**: global + switch; switch vence na mesma chave; VLAN `"0"` / `"00"` / `""` remove a chave no nível do switch.
2. **exclude_macs**: união, normalizada, deduplicada (MACs completos de 12 hex).
3. **exclude_ports**: só no switch; trim, lowercase, dedupe.
4. **allowed_vlans / protected_vlans**: listas mescladas, VLANs canônicas.
5. **multi_mac_policy**: switch herda o global; padrão `ignore`.
6. **Credenciais**: campos vazios do switch herdam o global (após expansão e password commands).

Arquivo de exemplo: [`docs/examples/config.yaml`](examples/config.yaml).

### Segredos

- `${VAR}` em `username` / `password` / `enable_password` (variável ausente → erro no Load).
- `password_command` / `enable_password_command` quando a senha está vazia: `/bin/sh -c` (ou `cmd /C`), timeout 5s, ambiente mínimo, trim de `\r\n` finais.
- `--ask-pass` pergunta uma vez e sobrescreve YAML/env para o(s) alvo(s) selecionado(s).

## Referência da CLI

```bash
negev (--target <ref>[,...] | --all | --group <tag>) [flags]
```

`--target`, `--all` e `--group` são mutuamente exclusivos.

| Flag | Descrição |
|---|---|
| `--target <ip\|name>[,...]` | IP(s) e/ou `name`(s) de switch |
| `--all` | Todos os switches do arquivo |
| `--group <tag>` | Switches cujas `tags` contêm `<tag>` |
| `--concurrency <n>` | Tamanho do pool de workers (padrão 4) |
| `--config <path>` | Caminho do YAML |
| `--write` | Aplica mudanças (padrão = sandbox) |
| `--verbose <0-3>` | 0 nenhum, 1 debug, 2 I/O bruto, 3 ambos |
| `--create-vlans` | Sincroniza VLANs com `allowed_vlans` |
| `--ask-pass` | Senhas interativas |
| `--backup` | Backup do running-config mesmo no sandbox |
| `--backup-dir <dir>` | Diretório de backup |
| `--audit-log <path>` | Caminho JSONL de auditoria (sobrescreve YAML) |
| `--version` | String de versão |

Códigos de saída: `0` OK / sem mudanças pendentes no sandbox, `1` erro em qualquer alvo, `2` mudanças simuladas no sandbox sem falhas.

A saída multi-run é bufferizada por alvo e liberada com prefixo `[label]`; uma tabela-resumo é impressa ao final.

## Plataformas suportadas

| Plataforma | YAML `platform` | Descoberta | VLAN de acesso | Save / dump de backup |
|---|---|---|---|---|
| Cisco IOS | `ios` | `show mac address-table`, `show interfaces status` | `switchport access vlan` | `write memory` / `show running-config` |
| Cisco NX-OS | `nxos` | `show mac address-table`, `show interface status` | `switchport` depois `switchport access vlan` | `copy running-config startup-config` / `show running-config` |
| Datacom DmSwitch 3000 | `dmswitch` (`dmos`) | `show mac-address-table`, status switchport | `set-member untagged` / native VLAN | `show running-config` |
| Datacom DM1200E | `dm1200` | `show mac-addr-table` | `vlan database`, `switchport access vlan`, `vlan participation exclude 1` | `show running-config` |
| 3Com / H3C Comware | `comware` | `display mac-address`, `display brief interface` | `port access vlan` | `save` / `display current-configuration` |

`platform: auto` sonda a saída de versão em ordem alfabética dos drivers registrados (Comware, DM1200, DmSwitch, IOS, NX-OS). A detecção IOS rejeita banners NX-OS.

## Como funciona

1. Resolve os switches selecionados (`ResolveTargets`).
2. Por switch (pool concorrente): carrega credenciais, detecta/obtém driver, aplica session profile + sequência de auth.
3. Se `--write` ou `--backup`: baixa o running-config e grava `<nome-ou-ip>_<UTC>.cfg` modo `0600` em `--backup-dir` / `backup_dir` / `./backups`. Falha de backup aborta antes de qualquer comando mutável.
4. Coleta VLANs, trunks, interfaces L3/SVI, portas ativas, tabela MAC.
5. Ignora trunks, portas roteadas, `exclude_ports`, MACs excluídos.
6. Resolve VLAN: maior prefixo de comprimento par em `mac_to_vlan` (12→2); senão `default_vlan`; portas vazias podem usar `no_data_vlan`.
7. Portas multi-MAC: `ignore` (pula em silêncio), `alert` (avisa + pula), `priority` (maior prefixo explícito vence).
8. `--create-vlans` opcional: cria allowed faltantes; remove não-allowed não-protegidas (1000–4094 sempre protegidas).
9. Sandbox imprime `SIMULATE:`; `--write` executa e salva.
10. Cada mudança de VLAN de porta (sucesso ou falha) anexa um registro JSONL quando `audit_log` / `--audit-log` está definido (modo `0644`, sync por entrada).

## Política multi-MAC

| Política | Comportamento |
|---|---|
| `ignore` (padrão) | Pula portas com mais de um MAC ativo; sem aviso |
| `alert` | Pula e registra cada MAC com a VLAN resolvida |
| `priority` | Entre hits explícitos de `mac_to_vlan`, escolhe o prefixo mais longo; se nenhum, pula |

## Backup e auditoria

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml --write --backup-dir ./backups
negev --target 192.0.2.10 --config docs/examples/config.yaml --backup   # sandbox + forçar backup
negev --target 192.0.2.10 --config docs/examples/config.yaml --write --audit-log ./audit/negev.jsonl
```

Campos de auditoria: `timestamp`, `user`, `target`, `switch_name`, `port`, `mac`, `old_vlan`, `new_vlan`, `action` (`SET_PORT_VLAN`), `sandbox`, `status`.

## Normalização de MAC

Separadores `:`, `.`, `-` são removidos; letras em minúsculas. Chaves de prefixo em `mac_to_vlan` devem ter comprimento par 2–12 hex. Listas de exclusão exigem MAC completo de 12 caracteres após normalização.

## Cenários práticos

### Cenário 1: Implantação inicial de switch de acesso

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml
negev --target 192.0.2.10 --config docs/examples/config.yaml --write
```

### Cenário 2: Sincronizar o banco de VLANs

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml --create-vlans
negev --target 192.0.2.10 --config docs/examples/config.yaml --create-vlans --write
```

### Cenário 3: Segmentar portas de acesso não gerenciadas

Mapeie prefixos (e MACs exatos) de departamento em `mac_to_vlan`, liste uplinks em `exclude_ports`, configure `multi_mac_policy` para portas telefone+PC, pré-visualize e use `--write`.

### Cenário 4: Diagnosticar prompts e pagers

```bash
negev --target 192.0.2.10 --config docs/examples/config.yaml --verbose 1
negev --target 192.0.2.10 --config docs/examples/config.yaml --verbose 2
negev --target 192.0.2.10 --config docs/examples/config.yaml --verbose 3
```

Use `--ask-pass` ou `password_command` quando as credenciais não devem ficar no YAML.
