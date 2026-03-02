BINARY_NAME := hash-diary
GO := go
GOFLAGS := -v
INSTALL_DIR := $(GOPATH)/bin

.PHONY: build test lint install clean

## build: Compile the binary
build:
	$(GO) build $(GOFLAGS) -o $(BINARY_NAME) .

## test: Run all tests
test:
	$(GO) test $(GOFLAGS) ./...

## lint: Run go vet
lint:
	$(GO) vet ./...

## install: Install the binary to GOPATH/bin
install:
	$(GO) install $(GOFLAGS) .

## clean: Remove build artifacts
clean:
	$(GO) clean
	rm -f $(BINARY_NAME)
