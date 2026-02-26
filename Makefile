# paths
makefile := $(realpath $(lastword $(MAKEFILE_LIST)))
cmd_dir  := ./cmd/imap-mcp
dist_dir := ./dist

# executables
GO    := go
MKDIR := mkdir -p

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
