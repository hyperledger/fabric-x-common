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

go_cmd          ?= go
go_test         ?= $(go_cmd) test -json -v -timeout 30m

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

GO_TEST_FMT_FLAGS := -hide empty-packages

## Run all tests
test: FORCE
	@$(go_test) ./... | gotestfmt ${GO_TEST_FMT_FLAGS}

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
	golangci-lint run --color=always --new-from-rev=main --timeout=4m
	@echo "Running License Header Linters..."
	scripts/license-lint.sh

PROTO_DIRS := $(shell find . -not -path "*/vendor/*" -not -path "*/.git/*" -name '*.proto' -print0 | xargs -0 -n 1 dirname | sort -u)

# 2. Define a phony target that runs the loop
.PHONY: proto
proto:
	@for dir in $(PROTO_DIRS); do \
		echo "Compiling protos in: $$dir"; \
		protoc --proto_path=. \
		--proto_path="/usr/include" \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		--go_out=paths=source_relative:. \
		$$dir/*.proto; \
	done
#
# PROTO_TARGETS ?= $(shell find ./api \
# 	 -name '*.proto' -print0 | \
# 	 xargs -0 -n 1 dirname | xargs -n 1 basename | \
# 	 sort -u | sed -e "s/^proto/proto-/" \
# )
#
# proto: $(PROTO_TARGETS)
#
# proto-%: FORCE
# 	@echo "Compiling: $*"
# 	@protoc --proto_path="${PWD}" \
#           --proto_path="/usr/include" \
#           --go-grpc_out=. --go-grpc_opt=paths=source_relative \
#           --go_out=paths=source_relative:. ${PWD}/api/proto$*/*.proto

# https://www.gnu.org/software/make/manual/html_node/Force-Targets.html
# If a rule has no prerequisites or recipe, and the target of the rule is a nonexistent file,
# then make imagines this target to have been updated whenever its rule is run.
# This implies that all targets depending on this one will always have their recipe run.
FORCE:
