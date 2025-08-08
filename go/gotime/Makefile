# GoTime (gt) - Makefile

# Variables
BINARY_NAME := gt
BUILD_DIR := .
INSTALL_DIR := $(HOME)/.local/bin
MODULE := github.com/sglavoie/dev-helpers/go/gotime

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod

# Build flags
LDFLAGS := -ldflags "-s -w"
BUILD_FLAGS := -trimpath $(LDFLAGS)

.PHONY: all build clean test deps install uninstall help

# Default target
all: build

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	$(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "Binary built: $(BUILD_DIR)/$(BINARY_NAME)"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	$(GOCLEAN)
	rm -f $(BUILD_DIR)/$(BINARY_NAME)
	@echo "Clean complete"

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "Dependencies updated"

# Install binary to user's local bin directory
install: build
	@echo "Installing $(BINARY_NAME) to $(INSTALL_DIR)..."
	@mkdir -p $(INSTALL_DIR)
	cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Installation complete: $(INSTALL_DIR)/$(BINARY_NAME)"
	@echo ""
	@echo "Make sure $(INSTALL_DIR) is in your PATH:"
	@echo "  export PATH=\"$(INSTALL_DIR):\$$PATH\""

# Uninstall binary from user's local bin directory
uninstall:
	@echo "Uninstalling $(BINARY_NAME) from $(INSTALL_DIR)..."
	rm -f $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Uninstallation complete"

# Build for multiple platforms
build-all: clean
	@echo "Building for multiple platforms..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 .
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe .
	@echo "Multi-platform build complete"

# Development build with race detection and debug info
dev:
	@echo "Building development version..."
	$(GOBUILD) -race -o $(BUILD_DIR)/$(BINARY_NAME)-dev .
	@echo "Development binary built: $(BUILD_DIR)/$(BINARY_NAME)-dev"

# Run the binary after building
run: build
	@echo "Running $(BINARY_NAME)..."
	./$(BINARY_NAME)

# Format code
fmt:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...
	@echo "Code formatting complete"

# Lint code (requires golangci-lint)
lint:
	@echo "Linting code..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Show version info
version:
	@echo "Module: $(MODULE)"
	@echo "Binary: $(BINARY_NAME)"
	@echo "Go version: $$($(GOCMD) version)"

# Show help
help:
	@echo "GoTime (gt) - Makefile Commands"
	@echo ""
	@echo "Available targets:"
	@echo "  build      - Build the gt binary"
	@echo "  install    - Build and install gt to ~/.local/bin"
	@echo "  clean      - Remove build artifacts"
	@echo "  test       - Run tests"
	@echo "  deps       - Download and tidy dependencies"
	@echo "  uninstall  - Remove gt from ~/.local/bin"
	@echo "  build-all  - Build for multiple platforms"
	@echo "  dev        - Build development version with race detection"
	@echo "  run        - Build and run gt"
	@echo "  fmt        - Format code"
	@echo "  lint       - Lint code (requires golangci-lint)"
	@echo "  version    - Show version information"
	@echo "  help       - Show this help message"
	@echo ""
	@echo "Examples:"
	@echo "  make build"
	@echo "  make install"
	@echo "  make clean install"