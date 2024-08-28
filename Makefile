# Project information
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
BIN_DIR := $(PROJECT_DIR)/bin

# Build binary
GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)
GOPATH=$(shell go env GOPATH)

BIN_EXT ?=

ifeq ($(GOOS),windows)
	BIN_EXT=.exe
	GOPATH := $(subst \,/,$(GOPATH))
endif
# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
GOBIN=$(GOPATH)/bin

# Git information
GIT_COMMIT = $(shell git rev-parse HEAD)
GIT_COMMIT_SHORT = $(shell git rev-parse --short HEAD)
GIT_TAG = $(shell git describe --tags --abbrev=0 --exact-match 2>/dev/null)
GIT_TREESTATE  = $(shell test -n "`git status --porcelain`" && echo "dirty" || echo "clean")
BUILDDATE = $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

## Set your version by env or using latest tags from git
VERSION ?= ""
ifeq ($(VERSION), "")
    ifeq ($(GIT_TAG),)
        # Forked repo may not sync tags from upstream, so give it a default tag to make CI happy.
        VERSION="v0.0.0"
    else
        VERSION=$(GIT_TAG)
    endif
endif

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make \033[36m<target>\033[0m\n"} /^[%\/a-zA-Z_._0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help

##@ Build

.PHONY: release
release: goreleaser ## Build pzip punzip archiver binary and publish.
	@if [ ! -f CHANGELOG-$(VERSION).md ]; then \
		echo "Error: CHANGELOG-$(VERSION).md does not exist."; \
		exit 1; \
  	fi
	$(GORELEASER) release --clean -f .goreleaser.yaml --release-notes CHANGELOG-$(VERSION).md

.PHONY: release-snapshot
release-snapshot: goreleaser ## Build pzip punzip archiver binary.
	$(GORELEASER) release --skip publish --snapshot --clean -f .goreleaser.yaml


##@ Install
GORELEASER = $(BIN_DIR)/goreleaser
.PHONY: goreleaser
goreleaser: ## Download goreleaser locally if necessary.
	$(call go-install-tool,$(GORELEASER),github.com/goreleaser/goreleaser/v2@latest)

# go-get-tool will 'go get' any package $2 and install it to $1.

define go-install-tool
@[ -f $(1)$(BIN_EXT) ] || { \
set -e ;\
echo "Downloading $(2) to $(BIN_DIR)" ;\
GOBIN=$(BIN_DIR) go install $(2) ;\
}
endef

define go-install-tool-global
@[ -f $(GOBIN)/$(1)$(BIN_EXT) ] || { \
set -e ;\
echo "Downloading $(2) to $(GOBIN)" ;\
go install $(2) ;\
}
endef