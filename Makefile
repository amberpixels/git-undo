# Variables
GOLANGCI_LINT := $(shell which golangci-lint)

PKGS := $(shell go list ./...)

BUILD_DIR := build
CMD_DIR = ./cmd/git-undo
MAIN_FILE := $(CMD_DIR)/main.go

BINARY_NAME := git-undo
INSTALL_DIR := $(shell go env GOPATH)/bin

# VERSION will be set when manually building from source
# pseudo_version will return the same format as the Go does.
VERSION  := $(shell ./scripts/pseudo_version.sh 2>/dev/null || echo "")

# Only add the flag when VERSION isn’t empty
LDFLAGS  := $(if $(strip $(VERSION)),-X "main.version=$(VERSION)")

# Default target
all: build

# Build the binary
.PHONY: build
build:
	@mkdir -p $(BUILD_DIR)
	@go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_FILE)

# Run the binary
.PHONY: run
run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

# Run tests
.PHONY: test
test:
	@go test -v ./...

# Run integration tests in dev mode (test current changes)
.PHONY: integration-test-dev  
integration-test-dev:
	@./scripts/run-integration.sh --dev

# Run integration tests in production mode (test real user experience)
.PHONY: integration-test-prod
integration-test-prod:
	@./scripts/run-integration.sh --prod

# Run integration tests (alias for dev mode)
.PHONY: integration-test
integration-test: integration-test-dev

# Run all tests (unit + integration dev)
.PHONY: test-all
test-all: test integration-test-dev

# Tidy: format and vet the code
.PHONY: tidy
tidy:
	@go fmt $(PKGS)
	@go vet $(PKGS)
	@go mod tidy

# Install golangci-lint only if it's not already installed
.PHONY: lint-install
lint-install:
	@if ! [ -x "$(GOLANGCI_LINT)" ]; then \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi

# Lint the code using golangci-lint
# todo reuse var if possible
.PHONY: lint
lint: lint-install
	$(shell which golangci-lint) run

# Install the binary globally with custom version info
.PHONY: binary-install
binary-install:
	@echo "Installing git-undo with version: $(VERSION)"
	@go install -ldflags "-X main.version=$(VERSION)" $(CMD_DIR)

.PHONY: install
install:
	./install.sh

.PHONY: uninstall
uninstall:
	./uninstall.sh

# Uninstall the binary and remove the alias
.PHONY: binary-uninstall
binary-uninstall:
	rm -f $(INSTALL_DIR)/$(BINARY_NAME)

.PHONY: buildscripts
buildscripts:
	@./scripts/build.sh

.PHONY: update
update:
	./update.sh
