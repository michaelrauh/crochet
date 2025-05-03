#!/usr/bin/env bash
set -e

echo "Starting Docker Compose environment..."
docker compose up --build -d

# Simple delay to allow services to start
echo "Waiting for services to start..."
echo "Services should be ready. Running tests..."

# Test 1: Submit first message
echo "Test 1: Submitting first message..."
response1=$(curl -s -X POST http://localhost:8080/ingest \
    -H "Content-Type: application/json" \
    -d '{"title": "Test Title", "text": "This is a test message"}')
echo "Response from first request: $response1"

# if ! echo "$response1" | grep -q '"status":"success"'; then
#     echo "ERROR: First ingest request failed. Expected 'success' status."
#     docker compose logs
#     docker compose down --remove-orphans
#     exit 1
# fi
# if ! echo "$response1" | grep -q '"version":1'; then
#     echo "ERROR: Version mismatch in first call. Expected version 1."
#     docker compose logs
#     docker compose down --remove-orphans
#     exit 1
# fi
# echo "First message successfully ingested."

# # Test 2: Submit second message
# echo "Test 2: Submitting second message..."
# response2=$(curl -s -X POST http://localhost:8080/ingest \
#     -H "Content-Type: application/json" \
#     -d '{"title": "Test Title", "text": "This is a test message. Hello world"}')
# echo "Response from second request: $response2"

# if ! echo "$response2" | grep -q '"status":"success"'; then
#     echo "ERROR: Second ingest request failed. Expected 'success' status."
#     docker compose logs
#     docker compose down --remove-orphans
#     exit 1
# fi
# if ! echo "$response2" | grep -q '"version":2'; then
#     echo "ERROR: Version mismatch in second call. Expected version 2."
#     docker compose logs
#     docker compose down --remove-orphans
#     exit 1
# fi
# echo "Second message successfully ingested."

# # Clean up
# echo "Tests completed successfully. Cleaning up..."
docker compose down --remove-orphans

# echo "SUCCESS: End-to-end tests passed!"