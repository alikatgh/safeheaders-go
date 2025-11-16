# Makefile for SafeHeaders-Go

.PHONY: help test test-race test-coverage lint fmt vet bench clean install-tools security fuzz examples

# Default target
help:
	@echo "SafeHeaders-Go - Development Makefile"
	@echo ""
	@echo "Available targets:"
	@echo "  make test           - Run all tests"
	@echo "  make test-race      - Run tests with race detector"
	@echo "  make test-coverage  - Run tests with coverage report"
	@echo "  make lint           - Run golangci-lint"
	@echo "  make fmt            - Format all Go code"
	@echo "  make vet            - Run go vet"
	@echo "  make bench          - Run benchmarks"
	@echo "  make security       - Run security scanners (gosec, govulncheck)"
	@echo "  make fuzz           - Run fuzz tests (requires -fuzz flag)"
	@echo "  make examples       - Run all examples"
	@echo "  make clean          - Clean build artifacts and caches"
	@echo "  make install-tools  - Install development tools"
	@echo "  make all            - Run fmt, vet, lint, test"
	@echo ""

# Run all tests
test:
	@echo "Running tests for all modules..."
	@for dir in jsmn-go stb-image-go stb-truetype-go tinyxml2-go cjson-go miniz-go cgltf-go dr-wav-go; do \
		echo "Testing $$dir..."; \
		(cd $$dir && go test -v ./...) || exit 1; \
	done
	@echo "✅ All tests passed!"

# Run tests with race detector
test-race:
	@echo "Running tests with race detector..."
	@for dir in jsmn-go stb-image-go stb-truetype-go tinyxml2-go cjson-go miniz-go cgltf-go dr-wav-go; do \
		echo "Testing $$dir (with -race)..."; \
		(cd $$dir && go test -race -v ./...) || exit 1; \
	done
	@echo "✅ All race tests passed!"

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@for dir in jsmn-go stb-image-go stb-truetype-go tinyxml2-go cjson-go miniz-go cgltf-go dr-wav-go; do \
		echo "Testing $$dir (with coverage)..."; \
		(cd $$dir && go test -coverprofile=coverage.out -covermode=atomic ./...) || exit 1; \
		(cd $$dir && go tool cover -func=coverage.out | grep total | awk '{print "Coverage: " $$3}'); \
	done
	@echo "✅ Coverage reports generated!"

# Run linter
lint:
	@echo "Running golangci-lint..."
	@for dir in jsmn-go stb-image-go stb-truetype-go tinyxml2-go cjson-go miniz-go cgltf-go dr-wav-go; do \
		echo "Linting $$dir..."; \
		(cd $$dir && golangci-lint run --config ../.golangci.yml) || exit 1; \
	done
	@echo "✅ Linting passed!"

# Format all code
fmt:
	@echo "Formatting Go code..."
	@go fmt ./...
	@echo "✅ Code formatted!"

# Run go vet
vet:
	@echo "Running go vet..."
	@for dir in jsmn-go stb-image-go stb-truetype-go tinyxml2-go cjson-go miniz-go cgltf-go dr-wav-go; do \
		(cd $$dir && go vet ./...) || exit 1; \
	done
	@echo "✅ go vet passed!"

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	@for dir in jsmn-go stb-image-go stb-truetype-go; do \
		echo "Benchmarking $$dir..."; \
		(cd $$dir && go test -bench=. -benchmem -benchtime=3s ./...) || true; \
	done
	@echo "✅ Benchmarks complete!"

# Run security scanners
security:
	@echo "Running security scanners..."
	@echo "Running gosec..."
	@gosec -fmt=text ./... || true
	@echo ""
	@echo "Running govulncheck..."
	@for dir in jsmn-go stb-image-go stb-truetype-go tinyxml2-go cjson-go miniz-go cgltf-go dr-wav-go; do \
		echo "Checking $$dir..."; \
		(cd $$dir && govulncheck ./...) || true; \
	done
	@echo "✅ Security scans complete!"

# Run fuzz tests (sample - run with specific target)
fuzz:
	@echo "Fuzz testing jsmn-go..."
	@echo "Note: Fuzz tests run indefinitely. Use Ctrl+C to stop."
	@echo "Example: cd jsmn-go && go test -fuzz=FuzzParse -fuzztime=30s"
	@(cd jsmn-go && go test -fuzz=FuzzParse -fuzztime=10s) || true

# Run examples
examples:
	@echo "Running examples..."
	@echo "Example 1: JSON Parser"
	@(cd examples/json-parser && go run main.go) || exit 1
	@echo ""
	@echo "✅ Examples executed successfully!"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@find . -name "coverage.out" -delete
	@find . -name "*.test" -delete
	@go clean -cache -testcache -modcache
	@echo "✅ Clean complete!"

# Install development tools
install-tools:
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.61.0
	@go install github.com/securego/gosec/v2/cmd/gosec@latest
	@go install golang.org/x/vuln/cmd/govulncheck@latest
	@go install golang.org/x/tools/cmd/benchstat@latest
	@echo "✅ Tools installed!"

# Run all checks (fmt, vet, lint, test)
all: fmt vet lint test
	@echo "✅ All checks passed!"

# Quick check before committing
pre-commit: fmt vet test-race
	@echo "✅ Pre-commit checks passed!"

# CI simulation (what runs in GitHub Actions)
ci: lint test-race test-coverage security
	@echo "✅ CI simulation complete!"

# Install module dependencies
deps:
	@echo "Downloading module dependencies..."
	@for dir in jsmn-go stb-image-go stb-truetype-go tinyxml2-go cjson-go miniz-go cgltf-go dr-wav-go; do \
		(cd $$dir && go mod download) || exit 1; \
	done
	@echo "✅ Dependencies downloaded!"

# Update module dependencies
update-deps:
	@echo "Updating module dependencies..."
	@for dir in jsmn-go stb-image-go stb-truetype-go tinyxml2-go cjson-go miniz-go cgltf-go dr-wav-go; do \
		echo "Updating $$dir..."; \
		(cd $$dir && go get -u ./... && go mod tidy) || exit 1; \
	done
	@echo "✅ Dependencies updated!"

# Generate test data
testdata:
	@echo "Generating test data..."
	@bash scripts/generate-testdata.sh
	@echo "✅ Test data generated!"

# Build all examples
build-examples:
	@echo "Building examples..."
	@mkdir -p bin
	@(cd examples/json-parser && go build -o ../../bin/json-parser main.go)
	@(cd examples/production-usage && go build -o ../../bin/production-server main.go)
	@echo "✅ Examples built in bin/ directory!"

# Show module information
info:
	@echo "SafeHeaders-Go Module Information"
	@echo "=================================="
	@echo ""
	@echo "Go Version:"
	@go version
	@echo ""
	@echo "Modules:"
	@for dir in jsmn-go stb-image-go stb-truetype-go tinyxml2-go cjson-go miniz-go cgltf-go dr-wav-go; do \
		echo "  - $$dir"; \
	done
	@echo ""
	@echo "Workspace:"
	@cat go.work
