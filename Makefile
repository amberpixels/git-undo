# Variables
GOLANGCI_LINT := $(shell which golangci-lint)

PKGS := $(shell go list ./...)

BUILD_DIR := build
CMD_UNDO_DIR = ./cmd/git-undo
CMD_BACK_DIR = ./cmd/git-back
UNDO_MAIN_FILE := $(CMD_UNDO_DIR)/main.go
BACK_MAIN_FILE := $(CMD_BACK_DIR)/main.go

UNDO_BINARY_NAME := git-undo
BACK_BINARY_NAME := git-back
INSTALL_DIR := $(shell go env GOPATH)/bin

# VERSION will be set when manually building from source
# pseudo_version will return the same format as the Go does.
VERSION  := $(shell ./scripts/src/pseudo_version.sh 2>/dev/null || echo "")

# Only add the flag when VERSION isn't empty
LDFLAGS  := $(if $(strip $(VERSION)),-X "main.version=$(VERSION)")

# Default target
all: build

# Build both binaries
.PHONY: build
build: build-undo build-back

# Build the git-undo binary
.PHONY: build-undo
build-undo:
	@mkdir -p $(BUILD_DIR)
	@go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(UNDO_BINARY_NAME) $(UNDO_MAIN_FILE)

# Build the git-back binary
.PHONY: build-back
build-back:
	@mkdir -p $(BUILD_DIR)
	@go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BACK_BINARY_NAME) $(BACK_MAIN_FILE)

# Run the git-undo binary
.PHONY: run
run: build-undo
	./$(BUILD_DIR)/$(UNDO_BINARY_NAME)

# Run the git-back binary
.PHONY: run-back
run-back: build-back
	./$(BUILD_DIR)/$(BACK_BINARY_NAME)

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

# Check shell scripts using shellcheck
.PHONY: sc
sc:
	@echo "Running shellcheck on all shell scripts..."
	@find scripts/ -name "*.sh" -o -name "*.bash" -o -name "*.zsh" | xargs shellcheck || true

# Install both binaries globally with custom version info
.PHONY: binary-install
binary-install: binary-install-undo binary-install-back

# Install git-undo binary globally
.PHONY: binary-install-undo
binary-install-undo:
	@echo "Installing git-undo with version: $(VERSION)"
	@go install -ldflags "$(LDFLAGS)" $(CMD_UNDO_DIR)

# Install git-back binary globally
.PHONY: binary-install-back
binary-install-back:
	@echo "Installing git-back with version: $(VERSION)"
	@go install -ldflags "$(LDFLAGS)" $(CMD_BACK_DIR)

# Install with support for verbose flag
.PHONY: install
install:
	./install.sh

# Install with verbose output
.PHONY: install-verbose
install-verbose:
	./install.sh --verbose

.PHONY: uninstall
uninstall:
	./uninstall.sh

# Uninstall both binaries
.PHONY: binary-uninstall
binary-uninstall:
	rm -f $(INSTALL_DIR)/$(UNDO_BINARY_NAME)
	rm -f $(INSTALL_DIR)/$(BACK_BINARY_NAME)

.PHONY: buildscripts
buildscripts:
	@./scripts/build.sh

.PHONY: update
update:
	./update.sh
