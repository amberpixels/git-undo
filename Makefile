# Variables
GOLANGCI_LINT := $(shell which golangci-lint)

BUILD_DIR := build
CMD_DIR = ./cmd/git-undo
MAIN_FILE := $(CMD_DIR)/main.go

BINARY_NAME := git-undo
INSTALL_DIR := $(shell go env GOPATH)/bin

# Default target
all: build

# Build the binary
build:
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_FILE)

# Run the binary
run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

# Run tests
test:
	@go test -v ./...

# Tidy: format and vet the code
tidy:
	@go fmt $$(go list ./...)
	@go vet $$(go list ./...)
	@go mod tidy

# Install golangci-lint only if it's not already installed
lint-install:
	@if ! [ -x "$(GOLANGCI_LINT)" ]; then \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi

# Lint the code using golangci-lint
# todo reuse var if possible
lint: lint-install
	$(shell which golangci-lint) run

# Install the binary globally with aliases
binary-install:
	@go install $(CMD_DIR)

install:
	./install.sh

# Uninstall the binary and remove the alias
binary-uninstall:
	rm -f $(INSTALL_DIR)/$(BINARY_NAME)

# Phony targets
.PHONY: all build run test tidy lint-install lint binary-install binary-uninstall install
