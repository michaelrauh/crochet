#!/usr/bin/env bash
set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m'

docker-compose up -d

# Test 1: Valid JSON
echo -e "${YELLOW}Test 1: Testing with valid JSON...${NC}"
response=$(curl -s -X POST http://localhost:8080/ingest \
  -H "Content-Type: application/json" \
  -d '{"title": "Test Title", "text": "This is a test message"}')

if echo "$response" | grep -q "success"; then
  echo -e "${GREEN}Test 1 passed! Received success response: $response${NC}"
else
  echo -e "${RED}Test 1 failed! Unexpected response: $response${NC}"
  docker-compose logs
  docker-compose down --remove-orphans
  exit 1
fi

# Test 2: Invalid JSON
invalid_response=$(curl -s -X POST http://localhost:8080/ingest \
  -H "Content-Type: application/json" \
  -d '{title: Invalid JSON}')

if echo "$invalid_response" | grep -q "Invalid JSON format"; then
  echo -e "${GREEN}Test 2 passed! Received expected error response: $invalid_response${NC}"
else
  echo -e "${RED}Test 2 failed! Unexpected response: $invalid_response${NC}"
  docker-compose logs
  docker-compose down --remove-orphans
  exit 1
fi

echo -e "${GREEN}All tests passed!${NC}"
docker-compose down --remove-orphans
exit 0
