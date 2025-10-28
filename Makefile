# Makefile for danksearch

BINARY_NAME=dsearch
SOURCE_DIR=cmd/dsearch
BUILD_DIR=bin
PREFIX ?= /usr/local
INSTALL_DIR=$(PREFIX)/bin
SYSTEMD_USER_DIR=$(HOME)/.config/systemd/user
SERVICE_NAME=dsearch

GO=go
GOFLAGS=-ldflags="-s -w"

VERSION=$(shell git describe --tags --always 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

BUILD_LDFLAGS=-ldflags="-s -w -X main.Version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.commit=$(COMMIT)"

.PHONY: all build clean install uninstall test fmt vet deps help install-service uninstall-service

all: build

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 $(GO) build $(BUILD_LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(SOURCE_DIR)/*.go
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

install: build
	@echo "Installing $(BINARY_NAME) to $(INSTALL_DIR)..."
	@mkdir -p $(INSTALL_DIR)
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@chmod +x $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Installation complete"

uninstall:
	@echo "Uninstalling $(BINARY_NAME) from $(INSTALL_DIR)..."
	@rm -f $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Uninstall complete"

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@echo "Clean complete"

test:
	@echo "Running tests..."
	$(GO) test -v ./...

fmt:
	@echo "Formatting Go code..."
	$(GO) fmt ./...

vet:
	@echo "Running go vet..."
	$(GO) vet ./...

deps:
	@echo "Updating dependencies..."
	$(GO) mod tidy
	$(GO) mod download

dev:
	@echo "Building $(BINARY_NAME) for development..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build -o $(BUILD_DIR)/$(BINARY_NAME) $(SOURCE_DIR)/*.go
	@echo "Development build complete: $(BUILD_DIR)/$(BINARY_NAME)"

check-go:
	@echo "Checking Go version..."
	@go version | grep -E "go1\.(2[2-9]|[3-9][0-9])" > /dev/null || (echo "ERROR: Go 1.22 or higher required" && exit 1)
	@echo "Go version OK"

version: check-go
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Commit: $(COMMIT)"

install-service:
	@echo "Installing systemd user service..."
	@mkdir -p $(SYSTEMD_USER_DIR)
	@cp assets/$(SERVICE_NAME).service $(SYSTEMD_USER_DIR)/$(SERVICE_NAME).service
	@systemctl --user daemon-reload
	@echo "Systemd service installed. Enable with: systemctl --user enable $(SERVICE_NAME)"
	@echo "Start with: systemctl --user start $(SERVICE_NAME)"

uninstall-service:
	@echo "Stopping and disabling $(SERVICE_NAME) service..."
	@systemctl --user stop $(SERVICE_NAME) 2>/dev/null || true
	@systemctl --user disable $(SERVICE_NAME) 2>/dev/null || true
	@rm -f $(SYSTEMD_USER_DIR)/$(SERVICE_NAME).service
	@systemctl --user daemon-reload
	@echo "Systemd service uninstalled"

help:
	@echo "Available targets:"
	@echo "  all              - Build the binary (default)"
	@echo "  build            - Build the binary"
	@echo "  install          - Install binary to $(INSTALL_DIR)"
	@echo "  uninstall        - Remove binary from $(INSTALL_DIR)"
	@echo "  clean            - Clean build artifacts"
	@echo "  test             - Run tests"
	@echo "  fmt              - Format Go code"
	@echo "  vet              - Run go vet"
	@echo "  deps             - Update dependencies"
	@echo "  dev              - Build with debug info"
	@echo "  check-go         - Check Go version compatibility"
	@echo "  version          - Show version information"
	@echo "  install-service  - Install systemd user service"
	@echo "  uninstall-service- Uninstall systemd user service"
	@echo "  help             - Show this help message"
