BINARY_NAME=hexyn-aws
BIN_DIR=$(shell pwd)/bin
GORELEASER=$(BIN_DIR)/goreleaser
GOIMPORTS=$(BIN_DIR)/goimports
LINTER=$(BIN_DIR)/golangci-lint
MOCKERY=$(BIN_DIR)/mockery

.PHONY: all tools mocks fmt lint test test-coverage check build build-all clean

all: fmt lint test build

mocks:
	@echo "Generating mocks..."
	$(MOCKERY)

tools:
	@echo "Installing tools to $(BIN_DIR)..."
	@mkdir -p $(BIN_DIR)
	GOBIN=$(BIN_DIR) go install github.com/goreleaser/goreleaser/v2@latest
	GOBIN=$(BIN_DIR) go install golang.org/x/tools/cmd/goimports@latest
	GOBIN=$(BIN_DIR) go install github.com/vektra/mockery/v3
	curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b $(BIN_DIR) v2.12.2

fmt:
	@echo "Formatting code..."
	$(GOIMPORTS) -w .
	gofmt -s -w .

lint:
	@echo "Running linter..."
	$(LINTER) run ./...

test:
	@echo "Running tests..."
	go test -v ./...

test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.txt -covermode=atomic ./...
	@grep -vE 'hexyn-aws/(test/|cmd/|main\.go)' coverage.txt > coverage.tmp && mv coverage.tmp coverage.txt
	go tool cover -func=coverage.txt

check:
	@echo "Checking GoReleaser configuration..."
	$(GORELEASER) check

build:
	@echo "Building for current OS..."
	@mkdir -p $(BIN_DIR)
	go build -ldflags="-s -w" -o $(BIN_DIR)/$(BINARY_NAME) main.go

build-all:
	@echo "Building for all platforms using GoReleaser (snapshot mode)..."
	$(GORELEASER) build --snapshot --clean

clean:
	@echo "Cleaning binaries and dist..."
	@rm -f $(BIN_DIR)/$(BINARY_NAME)
	@rm -rf dist/
	@rm -f coverage.txt coverage.tmp
