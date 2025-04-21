#!/usr/bin/env bash
set -e

# Run unit tests for the ingestor service
echo "Running unit tests for ingestor service..."
cd ingestor
go test -tags=test -v ./...
cd ..

# Run end-to-end test for the ingestor service
echo "Running end-to-end tests for ingestor service..."
bash test_e2e.sh
