# Makefile for Negev

BINARY_NAME = negev
CONFIG_FILE = config.yaml
GO = go
GOFLAGS = -v
BUILD_DIR = build
INSTALL_DIR_ROOT = /usr/local/bin
INSTALL_DIR_LOCAL = ~/.local/bin

.PHONY: all help build run run-write run-debug install deps clean

all: help

help:
	@printf "Make targets\n"
	@printf "  all            Show this help\n"
	@printf "  build          Build the binary into %s/\n" $(BUILD_DIR)
	@printf "  run            Execute in sandbox mode with %s\n" $(CONFIG_FILE)
	@printf "  run-write      Execute applying changes with %s\n" $(CONFIG_FILE)
	@printf "  run-debug      Execute with debug logging (verbosity 1)\n"
	@printf "  install        Copy the binary to %s or %s\n" $(INSTALL_DIR_ROOT) $(INSTALL_DIR_LOCAL)
	@printf "  deps           Fetch Go dependencies\n"
	@printf "  clean          Remove build artifacts\n"

build:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)

run:
	$(GO) run . -y $(CONFIG_FILE)

run-write:
	$(GO) run . -w -y $(CONFIG_FILE)

run-debug:
	$(GO) run . -v 1 -y $(CONFIG_FILE)

install: build
	@if [ "$$(id -u)" -eq 0 ]; then \
		echo "Installing $(BINARY_NAME) to $(INSTALL_DIR_ROOT)"; \
		cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_DIR_ROOT)/; \
	else \
		echo "Installing $(BINARY_NAME) to $(INSTALL_DIR_LOCAL)"; \
		mkdir -p $(INSTALL_DIR_LOCAL); \
		cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_DIR_LOCAL)/; \
	fi

deps:
	$(GO) get github.com/ziutek/telnet
	$(GO) get gopkg.in/yaml.v3

clean:
	rm -rf $(BUILD_DIR)
