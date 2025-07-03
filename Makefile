.PHONY: all build test integration-test e2e-test clean docker-build docker-up docker-down lint fmt help

# Variables
BINARY_NAME=mylock
DOCKER_IMAGE=mylock
GO_FILES=$(shell find . -name '*.go' -type f)

# Default target
all: test build

# Build the binary
build:
	go build -o $(BINARY_NAME) ./cmd/mylock

# Run unit tests
test:
	go test -v -race ./...

# Run integration tests (requires Docker)
integration-test: docker-up
	go test -v -tags=integration ./internal/locker/...
	$(MAKE) docker-down

# Run E2E tests (requires Docker)
e2e-test:
	./test/e2e_test.sh

# Run all tests
test-all: test integration-test e2e-test

# Format code
fmt:
	go fmt ./...
	gofmt -s -w .

# Lint code
lint:
	golangci-lint run
	go vet ./...

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -f mylock-*
	go clean

# Docker operations
docker-build:
	docker build -t $(DOCKER_IMAGE) .

docker-up:
	docker compose up -d
	@echo "Waiting for MySQL to be ready..."
	@for i in $$(seq 1 30); do \
		if docker compose exec -T mysql mysqladmin ping -h localhost -u root -prootpass >/dev/null 2>&1; then \
			echo "MySQL is ready!"; \
			break; \
		fi; \
		if [ $$i -eq 30 ]; then \
			echo "MySQL failed to start"; \
			exit 1; \
		fi; \
		sleep 1; \
	done

docker-down:
	docker compose down -v

# Install dependencies
deps:
	go mod download
	go mod tidy

# Build for multiple platforms
build-all:
	GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME)-linux-amd64 ./cmd/mylock
	GOOS=linux GOARCH=arm64 go build -o $(BINARY_NAME)-linux-arm64 ./cmd/mylock
	GOOS=darwin GOARCH=amd64 go build -o $(BINARY_NAME)-darwin-amd64 ./cmd/mylock
	GOOS=darwin GOARCH=arm64 go build -o $(BINARY_NAME)-darwin-arm64 ./cmd/mylock
	GOOS=windows GOARCH=amd64 go build -o $(BINARY_NAME)-windows-amd64.exe ./cmd/mylock

# Run the binary with example
run: build
	MYLOCK_HOST=localhost \
	MYLOCK_PORT=3306 \
	MYLOCK_USER=testuser \
	MYLOCK_PASSWORD=testpass \
	MYLOCK_DATABASE=testdb \
	./$(BINARY_NAME) --lock-name example --timeout 5 -- echo "Hello from mylock"

# Show help
help:
	@echo "Available targets:"
	@echo "  all              - Run tests and build"
	@echo "  build            - Build the binary"
	@echo "  test             - Run unit tests"
	@echo "  integration-test - Run integration tests (requires Docker)"
	@echo "  e2e-test         - Run E2E tests (requires Docker)"
	@echo "  test-all         - Run all tests"
	@echo "  fmt              - Format code"
	@echo "  lint             - Lint code"
	@echo "  clean            - Clean build artifacts"
	@echo "  docker-build     - Build Docker image"
	@echo "  docker-up        - Start MySQL container"
	@echo "  docker-down      - Stop MySQL container"
	@echo "  deps             - Install/update dependencies"
	@echo "  build-all        - Build for all platforms"
	@echo "  run              - Run example (requires MySQL)"
	@echo "  help             - Show this help"