# Makefile para o projeto Negev

# Variáveis
BINARY_NAME = negev
CONFIG_FILE = config.yaml
GO = go
GOFLAGS = -v

# Alvos padrão
.PHONY: all build run clean deps help

# Alvo padrão: compila o binário
all: build

# Compila o binário
build:
	$(GO) build $(GOFLAGS) -o $(BINARY_NAME)

# Executa o programa em modo sandbox com o arquivo de configuração padrão
run:
	$(GO) run . -y $(CONFIG_FILE)

# Executa o programa em modo execução (sem sandbox)
run-execute:
	$(GO) run . -x -y $(CONFIG_FILE)

# Executa o programa com depuração ativada
run-debug:
	$(GO) run . -d -y $(CONFIG_FILE)

# Instala as dependências
deps:
	$(GO) get github.com/ziutek/telnet
	$(GO) get gopkg.in/yaml.v3

# Limpa arquivos gerados
clean:
	rm -f $(BINARY_NAME)

# Exibe ajuda
help:
	@echo "Makefile para Negev"
	@echo ""
	@echo "Uso:"
	@echo "  make           # Compila o binário (equivalente a 'make build')"
	@echo "  make build     # Compila o binário"
	@echo "  make run       # Executa em modo sandbox com config.yaml"
	@echo "  make run-execute  # Executa em modo execução com config.yaml"
	@echo "  make run-debug # Executa com depuração e config.yaml"
	@echo "  make deps      # Instala as dependências"
	@echo "  make clean     # Remove o binário gerado"
	@echo "  make help      # Exibe esta ajuda"