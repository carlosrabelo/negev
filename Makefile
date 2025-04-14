# Makefile for Negev

# Variables
BINARY_NAME = negev
CONFIG_FILE = config.yaml
GO = go
GOFLAGS = -v
INSTALL_DIR_ROOT = /usr/local/bin
INSTALL_DIR_LOCAL = ~/.local/bin

# Default targets
.PHONY: all build run run-write run-debug install deps clean help

all: build

build:
	$(GO) build $(GOFLAGS) -o $(BINARY_NAME)

run:
	$(GO) run . -y $(CONFIG_FILE)

run-write:
	$(GO) run . -w -y $(CONFIG_FILE)

run-debug:
	$(GO) run . -d -y $(CONFIG_FILE)

install: build
	@if [ "$$(id -u)" -eq 0 ]; then \
		echo "Installing $(BINARY_NAME) to $(INSTALL_DIR_ROOT)"; \
		mv $(BINARY_NAME) $(INSTALL_DIR_ROOT)/; \
	else \
		echo "Installing $(BINARY_NAME) to $(INSTALL_DIR_LOCAL)"; \
		mkdir -p $(INSTALL_DIR_LOCAL); \
		mv $(BINARY_NAME) $(INSTALL_DIR_LOCAL)/; \
	fi

deps:
	$(GO) get github.com/ziutek/telnet
	$(GO) get gopkg.in/yaml.v3

clean:
	rm -f $(BINARY_NAME)

help:
	@echo "Makefile for Negev"
	@echo ""
	@echo "Usage:"
	@echo "  make           # Builds the binary"
	@echo "  make build     # Builds the binary"
	@echo "  make run       # Runs in sandbox mode with config.yaml"
	@echo "  make run-write # Runs in write mode with config.yaml"
	@echo "  make run-debug # Runs with debugging and config.yaml"
	@echo "  make install   # Installs the binary to /usr/local/bin (root) or ~/.local/bin (local user)"
	@echo "  make deps      # Installs dependencies"
	@echo "  make clean     # Removes the generated binary"
	@echo "  make help      # Displays this help"