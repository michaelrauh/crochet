#!/bin/bash

# Number of iterations to run
ITERATIONS=200

# Longer, more complex text to generate more work for the services
SAMPLE_TEXT="This is a much longer text sample that will require more processing. It includes various words and phrases that will generate different subphrases and require more CPU time to process. The goal is to create enough load to make profiling data more visible in Pyroscope dashboards. We want to ensure that block profiling and mutex profiling have enough events to record, since these are particularly important for diagnosing performance bottlenecks in Go services."

echo "Starting load test - sending $ITERATIONS requests to the ingestor service..."

for i in $(seq 1 $ITERATIONS); do
  # Add some randomness to the text to avoid any caching
  RANDOM_SUFFIX=$RANDOM
  
  # Send request with progress indication
  echo "Sending request $i of $ITERATIONS..."
  curl -s -X POST http://localhost:8080/ingest \
    -H "Content-Type: application/json" \
    -d "{\"title\": \"Load Test $i\", \"text\": \"$SAMPLE_TEXT $RANDOM_SUFFIX\"}" > /dev/null
  
  # Add a small random delay between requests (50-200ms)
  sleep 0.$(( 50 + RANDOM % 150 ))
done

echo "Load test completed - sent $ITERATIONS requests to generate profiling data"
echo "Check your Pyroscope dashboard at http://localhost:4040"