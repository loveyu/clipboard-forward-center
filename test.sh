#!/bin/bash
set -e

echo "Running tests..."
go test -race -v ./...
echo "All tests passed."
