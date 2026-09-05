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

echo "==> Running Go linter..."
if command -v golangci-lint &> /dev/null; then
    golangci-lint run ./...
elif [ -f "$HOME/go/bin/golangci-lint" ]; then
    "$HOME/go/bin/golangci-lint" run ./...
else
    echo "golangci-lint not found in PATH or ~/go/bin. Skipping golangci-lint."
fi

echo "==> Running OpenAPI 3.1 Spectral linter..."
if command -v spectral &> /dev/null; then
    spectral lint api/openapi.yaml --ruleset .spectral.yaml
elif command -v docker &> /dev/null; then
    docker run --rm -v "$(pwd)":/tmp stoplight/spectral lint /tmp/api/openapi.yaml --ruleset /tmp/.spectral.yaml
elif command -v npx &> /dev/null; then
    npx -y @stoplight/spectral-cli lint api/openapi.yaml --ruleset .spectral.yaml
else
    echo "Warning: spectral, docker, or npx not found in PATH. Skipping Spectral OpenAPI lint."
fi

echo "==> All linting completed successfully with zero errors."
