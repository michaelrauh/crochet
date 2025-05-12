#!/bin/bash
# Clean up any existing containers to ensure clean restart
echo "Cleaning up any existing containers..."
docker compose down

# Start services with rebuild
echo "Starting all services..."
docker compose up -d --build

# Wait for RabbitMQ to be healthy
echo "Waiting for RabbitMQ to be healthy..."
timeout=180
start_time=$(date +%s)
while true; do
  current_time=$(date +%s)
  elapsed=$((current_time - start_time))
  
  if [ $elapsed -gt $timeout ]; then
    echo "Timeout waiting for RabbitMQ to become healthy after $timeout seconds"
    break
  fi
  
  rabbitmq_status=$(docker compose ps rabbitmq | grep -o "healthy" || echo "")
  if [ "$rabbitmq_status" == "healthy" ]; then
    echo "RabbitMQ is now healthy after $elapsed seconds"
    break
  fi
  
  sleep 5
  echo "Still waiting for RabbitMQ... ($elapsed seconds elapsed)"
done

# Open monitoring dashboards
echo "Opening monitoring dashboards..."
open http://localhost:16686  # Jaeger UI for distributed tracing
open http://localhost:3000   # Grafana for metrics visualization
open http://localhost:4040   # Pyroscope for continuous profiling

echo "All services started. You can now use feed_princess.sh to send data."

