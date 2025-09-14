# Makefile for qf - Interactive Log Filter Composer
# =================================================

# Variables
BINARY_NAME := qf
MAIN_PATH := ./cmd/qf
BUILD_DIR := ./build
GO_VERSION := 1.25
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# Colors for output
RED := \033[31m
GREEN := \033[32m
YELLOW := \033[33m
BLUE := \033[34m
RESET := \033[0m

# Default target
.DEFAULT_GOAL := help

# Phony targets
.PHONY: help build install clean run dev watch test test-unit test-integration test-contract test-performance test-coverage test-verbose fmt lint vet check deps mod-tidy mod-verify docs validate

##@ Help

help: ## Display this help message
	@echo "$(BLUE)qf - Interactive Log Filter Composer$(RESET)"
	@echo "======================================"
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make $(YELLOW)<target>$(RESET)\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  $(YELLOW)%-17s$(RESET) %s\n", $$1, $$2 } /^##@/ { printf "\n$(BLUE)%s$(RESET)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Build

build: ## Build the qf binary
	@echo "$(GREEN)Building $(BINARY_NAME)...$(RESET)"
	@mkdir -p $(BUILD_DIR)
	@go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "$(GREEN)✓ Build complete: $(BUILD_DIR)/$(BINARY_NAME)$(RESET)"

install: build ## Install qf to system path
	@echo "$(GREEN)Installing $(BINARY_NAME)...$(RESET)"
	@go install $(LDFLAGS) $(MAIN_PATH)
	@echo "$(GREEN)✓ Installed $(BINARY_NAME) to $(shell go env GOPATH)/bin$(RESET)"

clean: ## Clean build artifacts
	@echo "$(YELLOW)Cleaning build artifacts...$(RESET)"
	@rm -rf $(BUILD_DIR)
	@go clean
	@echo "$(GREEN)✓ Clean complete$(RESET)"

##@ Development

run: ## Run the application directly
	@echo "$(GREEN)Running $(BINARY_NAME)...$(RESET)"
	@go run $(MAIN_PATH) $(ARGS)

dev: ## Run with debug output and test files
	@echo "$(GREEN)Running $(BINARY_NAME) in development mode...$(RESET)"
	@go run $(MAIN_PATH) --debug /tmp/qf-debug.log $(ARGS)

watch: ## Auto-rebuild on file changes (requires entr)
	@echo "$(GREEN)Watching for changes... (Press Ctrl+C to stop)$(RESET)"
	@find . -name "*.go" | entr -r make run

##@ Testing

test: ## Run all tests
	@echo "$(GREEN)Running all tests...$(RESET)"
	@go test -race ./...
	@echo "$(GREEN)✓ All tests passed$(RESET)"

test-unit: ## Run unit tests only
	@echo "$(GREEN)Running unit tests...$(RESET)"
	@go test -race ./tests/unit/...
	@echo "$(GREEN)✓ Unit tests passed$(RESET)"

test-integration: ## Run integration tests
	@echo "$(GREEN)Running integration tests...$(RESET)"
	@go test -race ./tests/integration/...
	@echo "$(GREEN)✓ Integration tests passed$(RESET)"

test-contract: ## Run contract tests
	@echo "$(GREEN)Running contract tests...$(RESET)"
	@go test -race ./tests/contract/...
	@echo "$(GREEN)✓ Contract tests passed$(RESET)"

test-performance: ## Run performance tests
	@echo "$(GREEN)Running performance tests...$(RESET)"
	@go test -race ./tests/performance/...
	@echo "$(GREEN)✓ Performance tests passed$(RESET)"

test-coverage: ## Generate test coverage report
	@echo "$(GREEN)Generating coverage report...$(RESET)"
	@mkdir -p $(BUILD_DIR)
	@go test -race -coverprofile=$(BUILD_DIR)/coverage.out ./...
	@go tool cover -html=$(BUILD_DIR)/coverage.out -o $(BUILD_DIR)/coverage.html
	@echo "$(GREEN)✓ Coverage report generated: $(BUILD_DIR)/coverage.html$(RESET)"

test-verbose: ## Run tests with verbose output
	@echo "$(GREEN)Running tests with verbose output...$(RESET)"
	@go test -race -v ./...

##@ Code Quality

fmt: ## Format Go code
	@echo "$(GREEN)Formatting Go code...$(RESET)"
	@gofmt -l -w .
	@echo "$(GREEN)✓ Code formatted$(RESET)"

lint: ## Run golangci-lint
	@echo "$(GREEN)Running linter...$(RESET)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
		echo "$(GREEN)✓ Linting complete$(RESET)"; \
	else \
		echo "$(YELLOW)golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest$(RESET)"; \
	fi

vet: ## Run go vet
	@echo "$(GREEN)Running go vet...$(RESET)"
	@go vet ./...
	@echo "$(GREEN)✓ Vet complete$(RESET)"

check: fmt vet lint ## Run all code quality checks
	@echo "$(GREEN)✓ All quality checks passed$(RESET)"

##@ Dependencies

deps: ## Download dependencies
	@echo "$(GREEN)Downloading dependencies...$(RESET)"
	@go mod download
	@echo "$(GREEN)✓ Dependencies downloaded$(RESET)"

mod-tidy: ## Clean up go.mod and go.sum
	@echo "$(GREEN)Tidying modules...$(RESET)"
	@go mod tidy
	@echo "$(GREEN)✓ Modules tidied$(RESET)"

mod-verify: ## Verify dependencies
	@echo "$(GREEN)Verifying modules...$(RESET)"
	@go mod verify
	@echo "$(GREEN)✓ Modules verified$(RESET)"

##@ Documentation

docs: ## Generate documentation
	@echo "$(GREEN)Generating documentation...$(RESET)"
	@mkdir -p $(BUILD_DIR)/docs
	@go doc -all ./... > $(BUILD_DIR)/docs/api.txt
	@echo "$(GREEN)✓ Documentation generated: $(BUILD_DIR)/docs/$(RESET)"

validate: ## Run validation scripts
	@echo "$(GREEN)Running validation scripts...$(RESET)"
	@if [ -f "./validate_core_functionality.go" ]; then \
		go run ./validate_core_functionality.go; \
		echo "$(GREEN)✓ Core functionality validation passed$(RESET)"; \
	fi
	@if [ -f "./simple_core_validation.go" ]; then \
		go run ./simple_core_validation.go; \
		echo "$(GREEN)✓ Simple core validation passed$(RESET)"; \
	fi

##@ Release

release-build: clean ## Build release binaries for multiple platforms
	@echo "$(GREEN)Building release binaries...$(RESET)"
	@mkdir -p $(BUILD_DIR)/release
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)
	@echo "$(GREEN)✓ Release binaries built in $(BUILD_DIR)/release/$(RESET)"

##@ Info

version: ## Show version information
	@echo "$(BLUE)Version:$(RESET) $(VERSION)"
	@echo "$(BLUE)Go Version:$(RESET) $(shell go version)"
	@echo "$(BLUE)Build Time:$(RESET) $(BUILD_TIME)"

env: ## Show build environment
	@echo "$(BLUE)Go Environment:$(RESET)"
	@go env | grep -E "(GOOS|GOARCH|GOVERSION|GOPATH|GOROOT)"