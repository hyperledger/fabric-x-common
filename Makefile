# Copyright IBM Corp All Rights Reserved.
# Copyright London Stock Exchange Group All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#
# -------------------------------------------------------------
# Run `make help` to find the supported targets

# Disable implicit rules
.SUFFIXES:
MAKEFLAGS += --no-builtin-rules

BUILD_DIR ?= bin

PKGNAME = github.com/hyperledger/fabric-x-common

GO_TAGS ?=

IMAGE_TAG ?= latest

TOOLS_EXES = configtxgen configtxlator cryptogen

pkgmap.configtxgen    := $(PKGNAME)/cmd/configtxgen
pkgmap.configtxlator  := $(PKGNAME)/cmd/configtxlator
pkgmap.cryptogen      := $(PKGNAME)/cmd/cryptogen

.DEFAULT_GOAL := help

MAKEFLAGS += --jobs=16

.PHONY: help
# List all commands with documentation
help: ## List all commands with documentation
	@echo "Available commands:"
	@awk 'BEGIN {FS = ":.*?## "}; /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: tools
tools: $(TOOLS_EXES) ## Builds all tools

## Run CMD and tools tests
test: FORCE
	go test -v ./cmd/...
	go test -v ./internaltools/...

.PHONY: $(TOOLS_EXES)
$(TOOLS_EXES): %: $(BUILD_DIR)/% ## Builds a native binary

$(BUILD_DIR)/%: GO_LDFLAGS = $(METADATA_VAR:%=-X $(PKGNAME)/common/metadata.%)
$(BUILD_DIR)/%:
	@echo "Building $@"
	@mkdir -p $(@D)
	@GOBIN=$(abspath $(@D)) go install -tags "$(GO_TAGS)" -ldflags "$(GO_LDFLAGS)" -buildvcs=false $(pkgmap.$(@F))
	@touch $@

.PHONY: clean
clean: ## Cleans the build area
	-@rm -rf $(BUILD_DIR)

lint: FORCE
	@echo "Running Go Linters..."
	golangci-lint run --color=always --new-from-rev=origin/main --timeout=4m
	@echo "Running License Header Linters..."
	scripts/license-lint.sh

# Build the fabric-x-tools image for the current machine platform.
.PHONY: build-fabric-x-tools-image
build-fabric-x-tools-image:
	./images/build_image.sh --tag docker.io/hyperledger/fabric-x-tools:$(IMAGE_TAG) -f ./images/Dockerfile

# Build the fabric-x-tools image for multiple platforms.
.PHONY: build-fabric-x-tools-multiplatform-image
build-fabric-x-tools-multiplatform-image:
	./images/build_image.sh --tag docker.io/hyperledger/fabric-x-tools:$(IMAGE_TAG) -f ./images/Dockerfile --multiplatform --push

# https://www.gnu.org/software/make/manual/html_node/Force-Targets.html
# If a rule has no prerequisites or recipe, and the target of the rule is a nonexistent file,
# then make imagines this target to have been updated whenever its rule is run.
# This implies that all targets depending on this one will always have their recipe run.
FORCE:
