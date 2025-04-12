# Makefile for the Negev project

# Variables
BINARY_NAME = negev
CONFIG_FILE = config.yaml
GO = go
GOFLAGS = -v

# Default targets
.PHONY: all build run clean deps help

# Default target: builds the binary
all: build

# Builds the binary
build:
	$(GO) build $(GOFLAGS) -o $(BINARY_NAME)

# Runs the program in sandbox mode with the default configuration file
run:
	$(GO) run . -y $(CONFIG_FILE)

# Runs the program in execution mode (without sandbox)
run-execute:
	$(GO) run . -x -y $(CONFIG_FILE)

# Runs the program with debugging enabled
run-debug:
	$(GO) run . -d -y $(CONFIG_FILE)

# Installs dependencies
deps:
	$(GO) get github.com/ziutek/telnet
	$(GO) get gopkg.in/yaml.v3

# Cleans generated files
clean:
	rm -f $(BINARY_NAME)

# Displays help
help:
	@echo "Makefile for Negev"
	@echo ""
	@echo "Usage:"
	@echo "  make           # Builds the binary (equivalent to 'make build')"
	@echo "  make build     # Builds the binary"
	@echo "  make run       # Runs in sandbox mode with config.yaml"
	@echo "  make run-execute  # Runs in execution mode with config.yaml"
	@echo "  make run-debug # Runs with debugging and config.yaml"
	@echo "  make deps      # Installs dependencies"
	@echo "  make clean     # Removes the generated binary"
	@echo "  make help      # Displays this help"