SHELL := /bin/sh

.PHONY: install format format-check lint test build verify clean

install:
	npm ci

format:
	npm run format

format-check:
	npm run format:check

lint:
	npm run lint

test:
	npm run test:coverage

build:
	npm run build

verify: format-check lint test build

clean:
	rm -rf dist coverage
