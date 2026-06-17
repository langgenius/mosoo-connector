GO ?= go
BUN ?= bun
LATHE ?= lathe
MOSOO_REPO ?= https://github.com/langgenius/mosoo.git
MOSOO_REF ?= main

GOBIN := $(shell $(GO) env GOBIN)
GOPATH := $(shell $(GO) env GOPATH)
BINDIR ?= $(if $(GOBIN),$(GOBIN),$(GOPATH)/bin)
MOSOO_DIR := .cache/mosoo
SPEC_FILE := docs/openapi/public-thread-api.openapi.json
SOURCE_NAME := threads
PINNED_TAG := local-snapshot
SYNC_DIR := .cache/specs-sync/$(SOURCE_NAME)

.DEFAULT_GOAL := help
.PHONY: help build install clean _codegen

help:
	@printf '%s\n' \
		'make build                  Generate and build bin/mosoo' \
		'make install                Install mosoo to $(BINDIR)/mosoo' \
		'make clean                  Remove generated files and caches' \
		'' \
		'Variables:' \
		'  MOSOO_REPO=$(MOSOO_REPO)' \
		'  MOSOO_REF=$(MOSOO_REF)' \
		'  BINDIR=$(BINDIR)'

build: _codegen
	cp cli.yaml cmd/mosoo/cli.yaml
	$(GO) build -trimpath -o bin/mosoo ./cmd/mosoo

install: build
	mkdir -p "$(BINDIR)"
	install -m 0755 bin/mosoo "$(BINDIR)/mosoo"

clean:
	rm -rf .cache bin cmd/mosoo/cli.yaml internal/generated skills specs/sources.yaml

_codegen:
	@mkdir -p .cache specs "$(SYNC_DIR)/docs/openapi"
	@if [ -d "$(MOSOO_DIR)/.git" ]; then \
		git -C "$(MOSOO_DIR)" fetch --all --tags --quiet; \
	else \
		git clone --quiet "$(MOSOO_REPO)" "$(MOSOO_DIR)"; \
	fi
	@if git -C "$(MOSOO_DIR)" rev-parse --verify --quiet "origin/$(MOSOO_REF)" >/dev/null; then \
		git -C "$(MOSOO_DIR)" checkout --quiet -B "$(MOSOO_REF)" "origin/$(MOSOO_REF)"; \
	else \
		git -C "$(MOSOO_DIR)" -c advice.detachedHead=false checkout --quiet "$(MOSOO_REF)"; \
	fi
	git -C "$(MOSOO_DIR)" submodule update --init --recursive
	cd "$(MOSOO_DIR)" && $(BUN) install --frozen-lockfile
	$(BUN) scripts/export-public-api-openapi.ts
	@printf '%s\n' \
		'sources:' \
		'  $(SOURCE_NAME):' \
		'    display_name: public-thread-api' \
		'    repo_url: file://$(CURDIR)/$(MOSOO_DIR)' \
		'    pinned_tag: $(PINNED_TAG)' \
		'    backend: openapi3' \
		'    openapi3:' \
		'      files:' \
		'        - $(SPEC_FILE)' \
		> specs/sources.yaml
	cp "$(MOSOO_DIR)/$(SPEC_FILE)" "$(SYNC_DIR)/$(SPEC_FILE)"
	@printf '%s\n' \
		'source: $(SOURCE_NAME)' \
		'backend: openapi3' \
		'synced_from: $(PINNED_TAG)' \
		'resolved_sha: $(PINNED_TAG)' \
		> "$(SYNC_DIR)/sync-state.yaml"
	$(LATHE) codegen -sources specs/sources.yaml -cache .cache
