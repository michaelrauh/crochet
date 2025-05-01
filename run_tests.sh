#!/usr/bin/env bash
set -e

# Use Go work mode
echo "Using workspace mode with Go 1.24.2..."
go work use

# Run unit tests for the ingestor service
echo "Running unit tests for ingestor service..."
cd ingestor
go test -tags=test -v ./...
cd ..

# Run unit tests for the remediations service
echo "Running unit tests for remediations service..."
cd remediations
go test -tags=test -v ./...
cd ..

# Run unit tests for the orthos service
echo "Running unit tests for orthos service..."
cd orthos
go test -tags=test -v ./...
cd ..

# Run unit tests for the workserver service
echo "Running unit tests for workserver service..."
cd workserver
go test -tags=test -v ./...
cd ..

# Run unit tests for the search service
echo "Running unit tests for search service..."
cd search
# Change from ./... pattern to specific test files
go test -tags=test -v .
cd ..

# Run unit tests for the clients package
echo "Running unit tests for clients package..."
cd clients
go mod tidy
go test -v .
cd ..

# Run end-to-end test for the ingestor service
echo "Running end-to-end tests for ingestor service..."
bash test_e2e.sh
echo "End-to-end tests completed successfully"
