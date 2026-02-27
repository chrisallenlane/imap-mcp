# paths
makefile := $(realpath $(lastword $(MAKEFILE_LIST)))
cmd_dir  := ./cmd/imap-mcp
dist_dir := ./dist

# executables
GO    := go
MKDIR := mkdir -p

# config
CONFIG ?= $(CURDIR)/config.toml

# build flags
BUILD_FLAGS := -ldflags="-s -w" -trimpath

## build: build an executable for your architecture
.PHONY: build
build: | clean $(dist_dir) fmt lint vet
	$(GO) build $(BUILD_FLAGS) -o $(dist_dir)/imap-mcp $(cmd_dir)

## install: build and install imap-mcp on your PATH
.PHONY: install
install: build
	$(GO) install $(BUILD_FLAGS) $(cmd_dir)

## setup: register imap-mcp with Claude Code
.PHONY: setup
setup: build
	@if [ ! -f "$(CONFIG)" ]; then \
		echo "Error: $(CONFIG) not found."; \
		echo "  cp config.example.toml config.toml"; \
		echo "  Edit config.toml with your IMAP credentials."; \
		exit 1; \
	fi
	claude mcp add imap-mcp \
		-s user \
		-- $(CURDIR)/dist/imap-mcp --config $(CONFIG)
	@echo ""
	@echo "Done. imap-mcp is now available in Claude Code."
	@echo "Verify with: claude mcp list"

## uninstall: unregister imap-mcp from Claude Code
.PHONY: uninstall
uninstall:
	claude mcp remove -s user imap-mcp
	@echo ""
	@echo "Done. imap-mcp has been removed from Claude Code."

## clean: remove compiled executables
.PHONY: clean
clean:
	rm -f $(dist_dir)/*

## fmt: format code with 80-column wrapping
.PHONY: fmt
fmt:
	$(GO) run github.com/segmentio/golines@latest -w --max-len=80 .
	$(GO) run mvdan.cc/gofumpt@latest -w .

## lint: lint go source files
.PHONY: lint
lint:
	$(GO) run github.com/mgechev/revive@latest ./...

## vet: vet go source files
.PHONY: vet
vet:
	$(GO) vet ./...

## test: run tests
.PHONY: test
test:
	$(GO) test ./...

## coverage: generate test coverage report
.PHONY: coverage
coverage:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out
	@echo ""
	@echo "HTML coverage report: coverage.html"
	$(GO) tool cover -html=coverage.out -o coverage.html

## fuzz: run fuzz tests (FUZZTIME=30s by default)
.PHONY: fuzz
fuzz:
	./scripts/fuzz.sh

## sloc: count source lines of code
.PHONY: sloc
sloc:
	scc --exclude-dir vendor .

## check: format, lint, vet, and test
.PHONY: check
check: | fmt lint vet test

# ./dist
$(dist_dir):
	$(MKDIR) $(dist_dir)

## help: display this help text
.PHONY: help
help:
	@cat $(makefile) | \
	sort             | \
	grep "^##"       | \
	sed 's/## //g'   | \
	column -t -s ':'
