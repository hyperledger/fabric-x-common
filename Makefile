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

go_cmd            ?= go
go_test           ?= $(go_cmd) test -json -v -timeout 30m
project_dir       := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
proto_flags       ?=
fabric_protos_tag ?= $(shell go list -m -f '{{.Version}}' github.com/hyperledger/fabric-protos-go-apiv2)

ifneq ("$(wildcard /usr/include)","")
    proto_flags += --proto_path="/usr/include"
endif

TOOLS_EXES = configtxgen configtxlator cryptogen

pkgmap.configtxgen    := $(PKGNAME)/cmd/configtxgen
pkgmap.configtxlator  := $(PKGNAME)/cmd/configtxlator
pkgmap.cryptogen      := $(PKGNAME)/cmd/cryptogen

.DEFAULT_GOAL := help

MAKEFLAGS += --jobs=16

.PHONY: help
## List all commands with documentation
help:
	@echo "Available commands:"
	@awk '\
       /^## / { h = substr($$0, 4); next } \
       /^[a-zA-Z_-]+:/ && h { \
         printf "\033[36m%-15s\033[0m %s\n", $$1, h; \
         h = "" \
       } \
     ' $(MAKEFILE_LIST)

.PHONY: tools
## Builds all tools
tools: $(TOOLS_EXES)

GO_TEST_FMT_FLAGS := -hide empty-packages

## Run all tests
test: FORCE
	@$(go_test) ./... | gotestfmt ${GO_TEST_FMT_FLAGS}

# Runs test with coverage analysis.
test-cover: FORCE
	@$(go_test) -coverprofile=coverage.profile -coverpkg=./... ./... | gotestfmt ${GO_TEST_FMT_FLAGS}
	@scripts/test-coverage-filter-files.sh

cover-report: FORCE
	$(go_cmd) tool cover -html=coverage.profile

.PHONY: $(TOOLS_EXES)
## Builds a native binary
$(TOOLS_EXES): %: $(BUILD_DIR)/%

$(BUILD_DIR)/%: GO_LDFLAGS = $(METADATA_VAR:%=-X $(PKGNAME)/common/metadata.%)
$(BUILD_DIR)/%:
	@echo "Building $@"
	@mkdir -p $(@D)
	@GOBIN=$(abspath $(@D)) go install -tags "$(GO_TAGS)" -ldflags "$(GO_LDFLAGS)" -buildvcs=false $(pkgmap.$(@F))
	@touch $@

.PHONY: clean
## Cleans the build area
clean:
	-@rm -rf $(BUILD_DIR)

## Run code linter
lint: lint-proto lint-asn1 FORCE
	@echo "Running Go Linters..."
	golangci-lint run --color=always --new-from-rev=main --timeout=4m
	@echo "Running License Header Linters..."
	scripts/license-lint.sh

## Run ASN.1 schema linter
lint-asn1: FORCE
	@echo "Running ASN.1 schema linters..."
	@asn1c -EP $(shell find ${project_dir}/api -name '*.asn')

# https://www.gnu.org/software/make/manual/html_node/Force-Targets.html
# If a rule has no prerequisites or recipe, and the target of the rule is a nonexistent file,
# then make imagines this target to have been updated whenever its rule is run.
# This implies that all targets depending on this one will always have their recipe run.
FORCE:

#########################
# Generate protos
#########################

BUILD_DIR := .build
PROTOS_REPO := https://github.com/hyperledger/fabric-protos.git
PROTOS_DIR := ${BUILD_DIR}/fabric-protos@${fabric_protos_tag}
# We depend on this specific file to ensure the repo is actually cloned
PROTOS_SENTINEL := ${PROTOS_DIR}/.git

## Build protobufs
proto: FORCE $(PROTOS_SENTINEL)
	@echo "Generating protobufs: $(shell find ${project_dir}/api -name '*.proto' -print0 \
    		| xargs -0 -n 1 dirname | xargs -n 1 basename | sort -u)"
	@protoc \
		-I="${project_dir}" \
		-I="${PROTOS_DIR}" \
		--go_opt=Mmsp/msp_config.proto=github.com/hyperledger/fabric-protos-go-apiv2/msp \
        --go-grpc_opt=Mmsp/msp_config.proto=github.com/hyperledger/fabric-protos-go-apiv2/msp \
		--go_opt=Mcommon/common.proto=github.com/hyperledger/fabric-protos-go-apiv2/common \
		--go_opt=Mcommon/ledger.proto=github.com/hyperledger/fabric-protos-go-apiv2/common \
		--go-grpc_opt=Mcommon/common.proto=github.com/hyperledger/fabric-protos-go-apiv2/common \
		--go-grpc_opt=Mcommon/ledger.proto=github.com/hyperledger/fabric-protos-go-apiv2/common \
		--go-grpc_out=. \
		--go-grpc_opt=paths=source_relative \
		--go_out=paths=source_relative:. \
		${proto_flags} \
		${project_dir}/api/*/*.proto


## Run protobuf linter
lint-proto: FORCE $(PROTOS_SENTINEL)
	@echo "Running protobuf linters..."
	@api-linter \
		-I="${project_dir}/api" \
		-I="${PROTOS_DIR}" \
		--config .apilinter.yaml \
		--set-exit-status \
		--output-format github \
		$(shell find ${project_dir}/api -name '*.proto' | sed 's|${project_dir}/api/||')

$(PROTOS_SENTINEL):
	@echo "Cloning fabric-protos..."
	@mkdir -p ${BUILD_DIR}
	@rm -rf ${PROTOS_DIR} # Ensure we start fresh if re-cloning
	@git -c advice.detachedHead=false clone --branch ${fabric_protos_tag} \
		--single-branch --depth 1 ${PROTOS_REPO} ${PROTOS_DIR}

## Generate testing mocks
mocks: FORCE
	@COUNTERFEITER_NO_GENERATE_WARNING=true go generate ./...

## Clean build dependencies
clean-deps:
	rm -rf ${PROTOS_DIR}
