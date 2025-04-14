MAKEFLAGS += --no-print-directory

.PHONY: build test lint fmt clean install uninstall help

BINARY_NAME := negev

build:
	./.make/build.sh

test:
	./.make/test.sh

lint:
	go vet ./...

fmt:
	go fmt ./...

clean:
	go clean
	rm -rf bin/

install: build
	./.make/install.sh

uninstall:
	./.make/uninstall.sh

help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  build      Build the binary"
	@echo "  test       Run tests"
	@echo "  lint       Run linter"
	@echo "  fmt        Format code"
	@echo "  clean      Remove build artifacts"
	@echo "  install    Install to ~/.local/bin"
	@echo "  uninstall  Remove from ~/.local/bin"
