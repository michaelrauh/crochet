#!/usr/bin/env bash
set -e

# Run unit tests
echo "Running unit tests..."
go test -v ./...

# Run end-to-end test
echo "Running end-to-end tests..."
bash test_e2e.sh
