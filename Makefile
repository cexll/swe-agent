.PHONY: help build run test test-coverage test-verbose clean fmt vet lint check docker-build docker-run tidy install-tools all vuln security ci

# Variables
BINARY_NAME=swe-agent
MAIN_PATH=cmd/main.go
DOCKER_IMAGE=swe-agent
DOCKER_TAG=latest
GO_VERSION?=1.25.1
CLAUDE_CLI_VERSION?=1.0.111
CODEX_CLI_VERSION?=0.23.0
DOCKER_BUILD_ARGS?=

# Default target
.DEFAULT_GOAL := help

## help: Display this help message
help:
	@echo "Available targets:"
	@echo ""
	@grep -E '^##' $(MAKEFILE_LIST) | sed 's/^## /  /'
	@echo ""

## build: Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) $(MAIN_PATH)
	@echo "Build complete: ./$(BINARY_NAME)"

## run: Run the application
run:
	@echo "Running application..."
	go run $(MAIN_PATH)

## test: Run all tests
test:
	@echo "Running tests..."
	go test ./...

## test-verbose: Run tests with verbose output
test-verbose:
	@echo "Running tests (verbose)..."
	go test -v ./...

## test-coverage: Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	go test -coverprofile=coverage.out ./...
	@echo ""
	@echo "Coverage summary:"
	go tool cover -func=coverage.out | tail -1
	@echo ""
	@echo "To view detailed HTML report, run: go tool cover -html=coverage.out"

## test-coverage-html: Generate HTML coverage report and open in browser
test-coverage-html: test-coverage
	@echo "Generating HTML coverage report..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "Report generated: coverage.html"

## fmt: Format Go code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	@echo "Formatting complete"

## vet: Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...
	@echo "Vet complete"

## lint: Run go vet and check formatting (strict; fails if unformatted)
lint: vet
	@echo "Checking code formatting..."
	@test -z "$$(gofmt -l .)" || (echo "Code is not formatted. Run 'make fmt'" && gofmt -l . && exit 1)
	@echo "Lint complete"

## fmt-check: Check formatting (non-fatal; prints unformatted files)
fmt-check:
	@echo "Checking code formatting (non-fatal)..."
	@if [ -n "$$(${SHELL} -lc 'gofmt -l .')" ]; then \
		echo "Unformatted files detected:"; \
		gofmt -l .; \
		echo "Tip: run 'make fmt' to auto-format"; \
	else \
		echo "All files are properly formatted"; \
	fi

## vuln: Run Go vulnerability scan (govulncheck)
vuln:
	@echo "Running govulncheck (basic security scan)..."
	@if ! command -v govulncheck >/dev/null 2>&1; then \
		echo "govulncheck not found. Installing..."; \
		GO111MODULE=on go install golang.org/x/vuln/cmd/govulncheck@latest; \
	fi
	@echo "govulncheck version:" && govulncheck -version || true
	govulncheck ./...
	@echo "Vulnerability scan complete"

## security: Alias for vuln
security: vuln

## check: Run all checks (fmt, vet, test)
check: fmt vet test
	@echo "All checks passed ✓"

## tidy: Tidy and verify dependencies
tidy:
	@echo "Tidying dependencies..."
	go mod tidy
	go mod verify
	@echo "Dependencies tidied"

## clean: Remove build artifacts and coverage files
clean:
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@rm -f coverage.out coverage.html
	@rm -f coverage_*.out
	@echo "Clean complete"

## docker-build: Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build \
		-t $(DOCKER_IMAGE):$(DOCKER_TAG) \
		--no-cache \
		--build-arg GO_VERSION=$(GO_VERSION) \
		--build-arg CLAUDE_CLI_VERSION=$(CLAUDE_CLI_VERSION) \
		--build-arg CODEX_CLI_VERSION=$(CODEX_CLI_VERSION) \
		$(DOCKER_BUILD_ARGS) \
		.
	@echo "Docker image built: $(DOCKER_IMAGE):$(DOCKER_TAG)"

## docker-run: Run Docker container (requires .env file)
docker-run:
	@echo "Running Docker container..."
	@test -f .env || (echo "Error: .env file not found" && exit 1)
	docker run -d -p 8000:8000 --env-file .env --name $(DOCKER_IMAGE) $(DOCKER_IMAGE):$(DOCKER_TAG)
	@echo "Container started: $(DOCKER_IMAGE)"
	@echo "Access at: http://localhost:8000"

## docker-stop: Stop and remove Docker container
docker-stop:
	@echo "Stopping Docker container..."
	@docker stop $(DOCKER_IMAGE) 2>/dev/null || true
	@docker rm $(DOCKER_IMAGE) 2>/dev/null || true
	@echo "Container stopped"

## docker-logs: View Docker container logs
docker-logs:
	@docker logs -f $(DOCKER_IMAGE)

## install-tools: Install development tools
install-tools:
	@echo "Installing development tools..."
	go install golang.org/x/tools/cmd/goimports@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
	@echo "Tools installed"

## all: Build, test, and run checks
all: clean check build
	@echo "Build complete and all checks passed ✓"

## ci: Run CI checks (no auto-format): lint, tests, build, security scan

ci:
	@echo "Running CI pipeline (lint, tests, build, security)..."
	$(MAKE) vet
	$(MAKE) fmt-check
	$(MAKE) test-coverage
	$(MAKE) build
	$(MAKE) vuln
	@echo "CI pipeline complete ✓"
