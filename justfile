#!/usr/bin/env -S just --justfile

binary := "deghosting"
pkg := "./cmd/deghosting"
bin_dir := "bin"

# encoding/json/v2 and encoding/json/jsontext are gated behind this experiment
# in Go 1.26. Exported so every recipe's go/golangci-lint invocation sees it.
export GOEXPERIMENT := "jsonv2"

# List available recipes
default:
    @just --list

# Build the CLI binary into ./bin
build:
    go build -o {{bin_dir}}/{{binary}} {{pkg}}

# Build and run the CLI, passing through any arguments
run *args:
    go run {{pkg}} {{args}}

# Install the CLI into $GOBIN / $GOPATH/bin
install:
    go install {{pkg}}

# Run the test suite
test *args:
    go test ./... {{args}}

# Run tests with the race detector and coverage
test-race:
    go test -race -cover ./...

# Run the golangci-lint linters
lint *args:
    golangci-lint run {{args}}

# Format all Go source with golangci-lint formatters
fmt:
    golangci-lint fmt

# Verify formatting without writing changes
fmt-check:
    golangci-lint fmt --diff

# Tidy and verify module dependencies
tidy:
    go mod tidy
    go mod verify

# Format, lint, and test
check: fmt-check lint test

# Remove build artifacts
clean:
    rm -rf {{bin_dir}}
    go clean
