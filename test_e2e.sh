#!/usr/bin/env bash
set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m'

docker-compose up -d --build

# Test round trip with valid JSON
echo -e "${YELLOW}Test 1: Testing round trip with valid JSON...${NC}"
response=$(curl -s -X POST http://localhost:8080/ingest \
    -H "Content-Type: application/json" \
    -d '{"title": "Test Title", "text": "This is a test message"}')

if echo "$response" | grep -q "success"; then
    echo -e "${GREEN}Test passed! Received success response: $response${NC}"
else
    echo -e "${RED}Test failed! Unexpected response: $response${NC}"
    echo -e "${YELLOW}Fetching logs for debugging...${NC}"
    docker-compose logs
    docker-compose down --remove-orphans
    exit 1
fi

docker-compose down --remove-orphans