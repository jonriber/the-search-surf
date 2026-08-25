SHELL := /bin/sh

API_BINARY := bin/api
MIGRATION_BINARY := bin/migrate
PACKAGES := ./...
GOCACHE ?= $(abspath ../.cache/go-build)
GOMODCACHE ?= $(abspath ../.cache/go-mod)
GOLANGCI_LINT_VERSION := v2.13.1
GOVULNCHECK_VERSION := v1.7.0

export GOCACHE
export GOMODCACHE

.PHONY: format format-check vet lint vulnerability-check test test-race build verify clean

format:
	gofmt -w .

format-check:
	@test -z "$$(gofmt -l .)" || { gofmt -l .; exit 1; }

vet:
	go vet $(PACKAGES)

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run $(PACKAGES)

vulnerability-check:
	go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) $(PACKAGES)

test:
	go test $(PACKAGES)

test-race:
	go test -race $(PACKAGES)

build:
	go build -trimpath -o $(API_BINARY) ./cmd/api
	go build -trimpath -o $(MIGRATION_BINARY) ./cmd/migrate

verify: format-check vet lint vulnerability-check test test-race build

clean:
	rm -f $(API_BINARY) $(MIGRATION_BINARY)
