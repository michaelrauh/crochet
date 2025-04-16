#!/usr/bin/env bash
set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m'

docker-compose up -d

echo -e "${YELLOW}Test 1: Testing with valid JSON...${NC}"
response=$(curl -s -X POST http://localhost:8080/ingest \
    -H "Content-Type: application/json" \
    -d '{"title": "Test Title", "text": "This is a test message"}')

if echo "$response" | grep -q "success"; then
    echo -e "${GREEN}Test passed! Received success response: $response${NC}"
    docker-compose down --remove-orphans
    exit 0  
else
    echo -e "${RED}Test failed! Unexpected response: $response${NC}"
    docker-compose logs
    docker-compose down --remove-orphans
    exit 1
fi

