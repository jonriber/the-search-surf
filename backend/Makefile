SHELL := /bin/sh

BINARY := bin/api
PACKAGES := ./...
GOCACHE ?= $(abspath ../.cache/go-build)
GOMODCACHE ?= $(abspath ../.cache/go-mod)

export GOCACHE
export GOMODCACHE

.PHONY: format format-check vet test test-race build verify clean

format:
	gofmt -w .

format-check:
	@test -z "$$(gofmt -l .)" || { gofmt -l .; exit 1; }

vet:
	go vet $(PACKAGES)

test:
	go test $(PACKAGES)

test-race:
	go test -race $(PACKAGES)

build:
	go build -trimpath -o $(BINARY) ./cmd/api

verify: format-check vet test test-race build

clean:
	rm -f $(BINARY)
