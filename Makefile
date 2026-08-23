SHELL := /bin/sh

.DEFAULT_GOAL := help

.PHONY: help repository-check backend-format backend-format-check backend-vet backend-test backend-test-race backend-build backend-verify verify

help: ## Show available repository commands
	@awk 'BEGIN {FS = ":.*## "; printf "Available commands:\n"} /^[a-zA-Z_-]+:.*## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

repository-check: ## Validate repository-level text and patch hygiene
	git diff --check

backend-format: ## Format Go backend source
	$(MAKE) -C backend format

backend-format-check: ## Verify Go backend formatting
	$(MAKE) -C backend format-check

backend-vet: ## Run Go static analysis
	$(MAKE) -C backend vet

backend-test: ## Run Go backend tests
	$(MAKE) -C backend test

backend-test-race: ## Run Go backend tests with race detection
	$(MAKE) -C backend test-race

backend-build: ## Build the Go API
	$(MAKE) -C backend build

backend-verify: ## Run every Go backend quality check
	$(MAKE) -C backend verify

verify: repository-check backend-verify ## Run every quality check currently implemented
