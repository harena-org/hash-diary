BINARY_NAME := hash-diary
GO := go
INSTALL_DIR := $(GOPATH)/bin

.PHONY: build test test-verbose lint install clean

## build: Compile the binary
build:
	$(GO) build -o $(BINARY_NAME) .

## test: Run all tests with race detector
test:
	$(GO) test -race ./...

## test-verbose: Run all tests with verbose output and race detector
test-verbose:
	$(GO) test -v -race ./...

## lint: Run go vet
lint:
	$(GO) vet ./...

## install: Install the binary to GOPATH/bin
install:
	$(GO) install .

## clean: Remove build artifacts
clean:
	$(GO) clean
	rm -f $(BINARY_NAME)
