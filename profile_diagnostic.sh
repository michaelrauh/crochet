#!/bin/bash

# Define colors for better output readability
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}==========================================${NC}"
echo -e "${BLUE}   Crochet System Profiling Diagnostic   ${NC}"
echo -e "${BLUE}==========================================${NC}"

# 1. Check if Pyroscope is running
echo -e "${YELLOW}Checking Pyroscope container status...${NC}"
PYROSCOPE_CONTAINER=$(docker ps | grep pyroscope | awk '{print $1}')

if [ -z "$PYROSCOPE_CONTAINER" ]; then
  echo -e "${YELLOW}Pyroscope container not found. Attempting to restart all services...${NC}"
  echo -e "${YELLOW}Stopping any existing containers...${NC}"
  docker-compose down 
  echo -e "${YELLOW}Starting all services with docker-compose...${NC}"
  docker-compose up -d
  
  echo -e "${YELLOW}Waiting 15 seconds for services to initialize...${NC}"
  sleep 15
else
  echo -e "${GREEN}Pyroscope container found: $PYROSCOPE_CONTAINER${NC}"
fi

# 2. Verify Pyroscope is accessible
echo -e "${YELLOW}Verifying Pyroscope is responding...${NC}"
PYROSCOPE_RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:4040/healthz)

if [ "$PYROSCOPE_RESPONSE" != "200" ]; then
  echo -e "${YELLOW}Pyroscope health check failed (HTTP $PYROSCOPE_RESPONSE). Restarting Pyroscope container...${NC}"
  docker restart $PYROSCOPE_CONTAINER
  echo -e "${YELLOW}Waiting 10 seconds for Pyroscope to restart...${NC}"
  sleep 10
  
  # Check again after restart
  PYROSCOPE_RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:4040/healthz)
  if [ "$PYROSCOPE_RESPONSE" != "200" ]; then
    echo -e "${YELLOW}Pyroscope still not responding correctly. There may be an issue with the container.${NC}"
  fi
else
  echo -e "${GREEN}Pyroscope is responding correctly (HTTP 200)${NC}"
fi

# 3. Generate load to create visible profiles
echo -e "${YELLOW}Generating load to create visible profiles...${NC}"
echo -e "${YELLOW}Running 300 requests with complex payloads...${NC}"

# Number of iterations to run
ITERATIONS=300

# Longer, more complex text to generate more work for the services
SAMPLE_TEXT="This is a much longer text sample that will require more processing. It includes various words and phrases that will generate different subphrases and require more CPU time to process. The goal is to create enough load to make profiling data more visible in Pyroscope dashboards. We want to ensure that block profiling and mutex profiling have enough events to record, since these are particularly important for diagnosing performance bottlenecks in Go services."

echo -e "${GREEN}Starting load test - sending $ITERATIONS requests to the ingestor service...${NC}"

for i in $(seq 1 $ITERATIONS); do
  # Add some randomness to the text to avoid any caching
  RANDOM_SUFFIX=$RANDOM
  
  # Only show progress every 10 requests to reduce output noise
  if [ $((i % 10)) -eq 0 ]; then
    echo -e "${BLUE}Progress: $i of $ITERATIONS requests sent${NC}"
  fi
  
  # Send request with varied payload size to ensure different processing patterns
  curl -s -X POST http://localhost:8080/ingest \
    -H "Content-Type: application/json" \
    -d "{\"title\": \"Load Test $i\", \"text\": \"$SAMPLE_TEXT $RANDOM_SUFFIX\"}" > /dev/null
  
  # Small random delay between requests (20-100ms)
  sleep 0.$(( 20 + RANDOM % 80 ))
done

echo -e "${GREEN}Load test completed - sent $ITERATIONS requests to generate profiling data${NC}"

# 4. Provide instructions for viewing the data
echo -e "${BLUE}==========================================${NC}"
echo -e "${GREEN}Profiling data should now be visible in Pyroscope.${NC}"
echo -e "${GREEN}Visit http://localhost:4040 to view the profiling dashboards.${NC}"
echo -e "${GREEN}Tips for using Pyroscope:${NC}"
echo -e "${BLUE}- Select the appropriate application (ingestor, context, or remediations)${NC}"
echo -e "${BLUE}- Choose a profile type (cpu, memory, block, mutex)${NC}"
echo -e "${BLUE}- Adjust the time range to include the period when load was generated${NC}"
echo -e "${BLUE}- Try the 'Timeline' view to see how metrics change over time${NC}"
echo -e "${BLUE}- Use the 'Diff' feature to compare different time periods${NC}"
echo -e "${BLUE}==========================================${NC}"
echo -e "${YELLOW}If profiles are still not visible, verify:${NC}"
echo -e "${YELLOW}1. The PYROSCOPE_PYROSCOPE_ENDPOINT environment variables in docker-compose.yml${NC}"
echo -e "${YELLOW}2. Check Docker network connectivity between services${NC}"
echo -e "${YELLOW}3. Review logs with: docker logs \$(docker ps | grep pyroscope | awk '{print \$1}')${NC}"
echo -e "${BLUE}==========================================${NC}"