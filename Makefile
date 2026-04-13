SHELL := /bin/sh

GO ?= go

PKG := ./cmd/kstoolchain
BIN_DIR ?= ./bin
BIN := $(BIN_DIR)/kstoolchain
INSTALL_BIN_DIR ?= $(shell gobin="$$($(GO) env GOBIN)"; if [ -n "$$gobin" ]; then echo "$$gobin"; else echo "$$($(GO) env GOPATH)/bin"; fi)
INSTALL_BIN := $(INSTALL_BIN_DIR)/kstoolchain

GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
BUILD_SOURCE ?= local
SOURCE_REPO ?= $(shell git rev-parse --show-toplevel 2>/dev/null || echo "")

LDFLAGS := -X github.com/kanini/keystone-toolchain/internal/contract.BuildCommit=$(GIT_COMMIT) \
           -X github.com/kanini/keystone-toolchain/internal/contract.BuildDate=$(BUILD_DATE) \
           -X github.com/kanini/keystone-toolchain/internal/contract.BuildSource=$(BUILD_SOURCE) \
           -X github.com/kanini/keystone-toolchain/internal/contract.SourceRepo=$(SOURCE_REPO)

.PHONY: help dev build install deps tidy test clean

help:
	@echo "make dev ARGS='version --json'  # run current source"
	@echo "make build                      # build ./bin/kstoolchain with provenance"
	@echo "make install                    # install kstoolchain to PATH with provenance"
	@echo "make deps                       # download modules without rewriting go.mod/go.sum"
	@echo "make tidy                       # run go mod tidy explicitly"
	@echo "make test                       # run go test ./..."

dev:
	$(GO) run $(PKG) $(ARGS)

build:
	mkdir -p $(BIN_DIR)
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN) $(PKG)
	@$(BIN) version

install:
	$(GO) install -ldflags "$(LDFLAGS)" $(PKG)
	@echo "Installed: $(INSTALL_BIN)"
	@$(INSTALL_BIN) version
	@path_bin="$$(command -v kstoolchain || true)"; \
	if [ -n "$$path_bin" ]; then \
		echo "PATH: $$path_bin"; \
		if [ "$$path_bin" != "$(INSTALL_BIN)" ]; then \
			echo "Note: PATH resolves kstoolchain to $$path_bin; installed binary is $(INSTALL_BIN)."; \
		fi; \
	else \
		echo "Note: $(INSTALL_BIN_DIR) is not on PATH."; \
	fi

deps:
	$(GO) mod download

tidy:
	$(GO) mod tidy

test:
	$(GO) test ./...

clean:
	rm -f $(BIN)

