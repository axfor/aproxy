.PHONY: all build clean test test-unit test-integration test-pg-unsupported bench run docker-build docker-push help

BINARY_NAME=aproxy
VERSION?=dev
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

GOEXPERIMENT=greenteagc
export GOEXPERIMENT

all: build

help:
	@echo "Available targets:"
	@echo "  build              - Build the binary"
	@echo "  clean              - Remove build artifacts"
	@echo "  test               - Run all tests (unit + integration)"
	@echo "  test-unit          - Run unit tests"
	@echo "  test-integration   - Run integration tests (PG-supported features)"
	@echo "  test-pg-unsupported- Run tests for PG-unsupported MySQL features (most will skip)"
	@echo "  bench              - Run benchmarks"
	@echo "  run                - Build and run the proxy"
	@echo "  docker-build       - Build Docker image"
	@echo "  docker-push        - Push Docker image"

build:
	@echo "Building $(BINARY_NAME) with GOEXPERIMENT=$(GOEXPERIMENT)..."
	GOEXPERIMENT=$(GOEXPERIMENT) go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/aproxy

build-linux:
	@echo "Building $(BINARY_NAME) for Linux with GOEXPERIMENT=$(GOEXPERIMENT)..."
	GOOS=linux GOARCH=amd64 GOEXPERIMENT=$(GOEXPERIMENT) go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 ./cmd/aproxy

clean:
	@echo "Cleaning..."
	rm -rf bin/
	go clean

test: test-unit test-integration

test-unit:
	@echo "Running unit tests with GOEXPERIMENT=$(GOEXPERIMENT)..."
	GOEXPERIMENT=$(GOEXPERIMENT) go test -v -race -coverprofile=coverage.out ./pkg/... ./internal/...

test-integration: build
	@echo "Running integration tests with GOEXPERIMENT=$(GOEXPERIMENT)..."
	@echo "Starting aproxy service..."
	@lsof -ti:3306 | xargs kill -9 2>/dev/null || true
	@lsof -ti:9090 | xargs kill -9 2>/dev/null || true
	@./bin/$(BINARY_NAME) -config configs/config.yaml > /tmp/aproxy-test.log 2>&1 & echo $$! > /tmp/aproxy-test.pid
	@echo "Waiting for aproxy to be ready..."
	@sleep 3
	@echo "Running tests..."
	@GOEXPERIMENT=$(GOEXPERIMENT) go test -v -timeout 5m ./test/integration/... || (kill `cat /tmp/aproxy-test.pid` 2>/dev/null; rm -f /tmp/aproxy-test.pid; exit 1)
	@echo "Stopping aproxy service..."
	@kill `cat /tmp/aproxy-test.pid` 2>/dev/null || true
	@rm -f /tmp/aproxy-test.pid

test-pg-unsupported: build
	@echo "Running PostgreSQL-unsupported MySQL feature tests with GOEXPERIMENT=$(GOEXPERIMENT)..."
	@echo "NOTE: Most tests will be skipped (t.Skip) as these features are not supported by PostgreSQL"
	@echo "Starting aproxy service..."
	@lsof -ti:3306 | xargs kill -9 2>/dev/null || true
	@lsof -ti:9090 | xargs kill -9 2>/dev/null || true
	@./bin/$(BINARY_NAME) -config configs/config.yaml > /tmp/aproxy-test.log 2>&1 & echo $$! > /tmp/aproxy-test.pid
	@echo "Waiting for aproxy to be ready..."
	@sleep 3
	@echo "Running tests..."
	@GOEXPERIMENT=$(GOEXPERIMENT) go test -v -timeout 5m ./test/pg-unsupported/... || (kill `cat /tmp/aproxy-test.pid` 2>/dev/null; rm -f /tmp/aproxy-test.pid; exit 1)
	@echo "Stopping aproxy service..."
	@kill `cat /tmp/aproxy-test.pid` 2>/dev/null || true
	@rm -f /tmp/aproxy-test.pid

bench:
	@echo "Running benchmarks with GOEXPERIMENT=$(GOEXPERIMENT)..."
	GOEXPERIMENT=$(GOEXPERIMENT) go test -bench=. -benchmem ./pkg/... ./internal/...

run: build
	@echo "Running $(BINARY_NAME)..."
	./bin/$(BINARY_NAME) -config configs/config.yaml

docker-build:
	@echo "Building Docker image..."
	docker build -t aproxy:$(VERSION) -f deployments/docker/Dockerfile .

docker-push:
	@echo "Pushing Docker image..."
	docker push aproxy:$(VERSION)

install-deps:
	@echo "Installing dependencies..."
	go mod download
	go mod verify

lint:
	@echo "Running linters..."
	golangci-lint run ./...

fmt:
	@echo "Formatting code..."
	go fmt ./...
	goimports -w .

mod-tidy:
	@echo "Tidying go.mod..."
	go mod tidy

coverage:
	@echo "Generating coverage report..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

.DEFAULT_GOAL := help
