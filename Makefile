.PHONY: all build test lint run clean

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
BINARY_NAME=bin/aether
MAIN_PKG=./cmd/aether

all: lint test build

build:
	$(GOBUILD) -o $(BINARY_NAME) $(MAIN_PKG)

test:
	$(GOTEST) -v -count=1 ./internal/...

test-short:
	$(GOTEST) -short ./internal/...

lint:
	golangci-lint run ./...

run: build
	./$(BINARY_NAME)

clean:
	rm -f $(BINARY_NAME)
	rm -f data/aether.db*
