# The Docker build can't see .git, so the version is computed here and passed in.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
export VERSION

.PHONY: up build version

up:
	docker compose up -d --build

build:
	docker compose build

version:
	@echo $(VERSION)
