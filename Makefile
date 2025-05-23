# Variables
GOLANGCI_LINT := $(shell which golangci-lint)

PKGS := $(shell go list ./...)

BUILD_DIR := build
CMD_DIR = ./cmd/git-undo
MAIN_FILE := $(CMD_DIR)/main.go

BINARY_NAME := git-undo
INSTALL_DIR := $(shell go env GOPATH)/bin

# Default target
all: build

# Build the binary
.PHONY: build
build:
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_FILE)

# Run the binary
.PHONY: run
run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

# Run tests
.PHONY: test
test:
	@go test -v ./...

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

# Install the binary globally with aliases
.PHONY: binary-install
binary-install:
	@go install $(CMD_DIR)

.PHONY: install
install:
	./install.sh

# Uninstall the binary and remove the alias
.PHONY: binary-uninstall
binary-uninstall:
	rm -f $(INSTALL_DIR)/$(BINARY_NAME)
