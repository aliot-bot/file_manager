.PHONY: run build test clean lint help

BINARY_NAME=file-manager
MAIN_PATH=./cmd/main.go
CONFIG_FILE=config.yaml

run: 
	@echo "Starting file manager..."
	@docker-compose up --build -d

down:
	@echo "Kill docker image..."
	@docker-compose down

stan_run:
	@echo "Starting file manager..."
	@go run $(MAIN_PATH)

build:
	@echo "Building $(BINARY_NAME)..."
	@go build -o $(BINARY_NAME) $(MAIN_PATH)
	@echo "Build complete: $(BINARY_NAME)"

test:
	@echo "Running tests..."
	@go test ./... -v

test-coverage:
	@echo "Running tests with coverage..."
	@go test ./... -coverprofile=coverage.out
	@go tool cover -func=coverage.out

lint:
	@echo "Running linter..."
	@./bin/golangci-lint run ./...

clean:
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@rm -f coverage.out
	@go clean
	@echo "Clean complete"

deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy


