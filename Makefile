SHELL := /bin/sh

.DEFAULT_GOAL := help

.PHONY: help repository-check verify

help: ## Show available repository commands
	@awk 'BEGIN {FS = ":.*## "; printf "Available commands:\n"} /^[a-zA-Z_-]+:.*## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

repository-check: ## Validate repository-level text and patch hygiene
	git diff --check

verify: repository-check ## Run every quality check currently implemented

