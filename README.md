# Negev

[![Go Report Card](https://goreportcard.com/badge/github.com/carlosrabelo/negev)](https://goreportcard.com/report/github.com/carlosrabelo/negev)

**Negev** é uma ferramenta de automação de VLANs em switches Cisco via Telnet. Atribui dinamicamente VLANs com base em prefixos de MAC, usando um modelo flexível e de fácil configuração.

---

## 🚀 Principais Funcionalidades

- Conexão via Telnet com switches Cisco
- Identificação de dispositivos pela tabela de MAC dinâmica
- Atribuição automática de VLANs com base em prefixos de MAC
- Modo **sandbox** para simulação segura
- Persistência das configurações com `write memory`
- Substituição dinâmica de VLANs via CLI
- Exclusão de MACs específicos da reconfiguração
- Detecção e exclusão automática de interfaces trunk

---

## 🔧 Instalação

```bash
git clone https://github.com/carlosrabelo/negev.git
cd negev
go build -o negev main.go
```

## 📂 Estrutura do YAML de Configuração

Este é o arquivo que define o comportamento do Negev, incluindo VLAN padrão, mapeamentos por prefixo MAC e exclusões.

```bash
host: "192.168.1.1"
username: "admin"
password: "senha"
enable_password: "senha_enable"
default_vlan: "10"

mac_to_vlan:
  "3c:2a:f4": "30"  # Brother
  "dc:c2:c9": "30"  # Canon
  "00:c8:8b": "50"  # Cisco AP

exclude_macs:
  - "d8:d3:85:d7:0d:b7"
  - "ac:16:2d:34:bb:da"
```

Campos obrigatórios:

- host: IP do switch Cisco
- username/password/enable_password: credenciais Telnet e do modo privilegiado
- default_vlan: usada para MACs não mapeados
- mac_to_vlan: mapeamento de prefixos MAC (3 primeiros bytes) para VLANs
- exclude_macs: MACs completos a serem ignorados

## 📌 Exemplos de Uso

```bash
negev -y example.yaml
```

## 🚀 Execução Real (Aplicar Configurações no Switch)

```bash
negev -y example.yaml -x
```

Aplica as VLANs configuradas e salva com write memory.

## 🔄 Substituir VLAN Dinamicamente

```bash
negev -y example.yaml -x -rv 10,100
```

## 🐞 Modo Execução com Saída de Debug

```bash
negev -y example.yaml -x -d
```

## 💻 Sobrescrever Host do YAML

```bash
./negev -y example.yaml -rv 10,99
```

## ⚠️ Aviso de Segurança

- Telnet é um protocolo inseguro; use apenas em redes confiáveis.
- O Negev não solicita confirmação antes de aplicar mudanças.
- Utilize o modo sandbox (default) para testar antes de usar **-x**.

## 📎 Contribuindo

Pull requests são bem-vindos. Sugestões para suporte a SSH, autenticação 802.1X ou interfaces web são encorajadas.
