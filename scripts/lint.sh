#!/usr/bin/env bash
set -e

echo "==> Running go fmt check..."
UNFORMATTED=$(gofmt -l .)
if [ -n "$UNFORMATTED" ]; then
    echo "The following files are not formatted properly:"
    echo "$UNFORMATTED"
    echo "Run 'go fmt ./...' to fix."
    exit 1
fi

echo "==> Running go vet..."
go vet ./...

echo "==> Running linter..."
if command -v golangci-lint &> /dev/null; then
    golangci-lint run ./...
elif [ -f "$HOME/go/bin/golangci-lint" ]; then
    "$HOME/go/bin/golangci-lint" run ./...
else
    echo "golangci-lint not found in PATH or ~/go/bin. Skipping golangci-lint."
fi

echo "==> Linting completed successfully with zero errors."
