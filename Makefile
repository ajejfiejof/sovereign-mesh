.PHONY: all build test coverage clean run keygen

BINARY_NAME=sovereign-mesh
GO_FILES=$(shell find . -name "*.go" -not -path "./vendor/*")

all: build test

build:
	@echo "Building $(BINARY_NAME) with stripped symbols..."
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BINARY_NAME) ./cmd/sovereign-mesh
	@echo "Build complete: $(BINARY_NAME)"

test:
	@echo "Running test suite..."
	go test -v ./...

coverage:
	@echo "Generating coverage report..."
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

run: build
	@echo "Starting Sovereign Mesh node..."
	./$(BINARY_NAME) node

keygen: build
	./$(BINARY_NAME) keygen

clean:
	@echo "Cleaning artifacts..."
	rm -f $(BINARY_NAME) coverage.out
	rm -rf .sovereign-mesh/
