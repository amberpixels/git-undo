# Variables
GOLANGCI_LINT := $(shell which golangci-lint)

PKGS := $(shell go list ./...)

BUILD_DIR := build
CMD_DIR = ./cmd/git-undo
MAIN_FILE := $(CMD_DIR)/main.go

BINARY_NAME := git-undo
INSTALL_DIR := $(shell go env GOPATH)/bin

# Build version with git information
VERSION_TAG := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
VERSION_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
VERSION_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
VERSION_DATE := $(shell date +%Y%m%d%H%M%S)

# Conditionally include branch in version string
ifeq ($(VERSION_BRANCH),main)
VERSION := $(VERSION_TAG)-$(VERSION_DATE)-$(VERSION_COMMIT)
else ifeq ($(VERSION_BRANCH),unknown)
VERSION := $(VERSION_TAG)-$(VERSION_DATE)-$(VERSION_COMMIT)
else
VERSION := $(VERSION_TAG)-$(VERSION_DATE)-$(VERSION_COMMIT)-$(VERSION_BRANCH)
endif

# Default target
all: build

# Build the binary
.PHONY: build
build:
	@mkdir -p $(BUILD_DIR)
	@go build -ldflags "-X main.version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_FILE)

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
