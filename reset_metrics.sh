#!/bin/bash

echo "Stopping all containers..."
docker-compose down

echo "Removing Prometheus data..."
# Find and remove the prometheus data volume
PROMETHEUS_VOLUME=$(docker volume ls -q | grep prometheus)
if [ -n "$PROMETHEUS_VOLUME" ]; then
    docker volume rm $PROMETHEUS_VOLUME
    echo "Prometheus volume removed"
else
    echo "No Prometheus volume found, continuing..."
fi

echo "Removing Grafana data..."
# Find and remove the grafana data volume
GRAFANA_VOLUME=$(docker volume ls -q | grep grafana)
if [ -n "$GRAFANA_VOLUME" ]; then
    docker volume rm $GRAFANA_VOLUME
    echo "Grafana volume removed"
else
    echo "No Grafana volume found, continuing..."
fi

# In case there are unnamed volumes, prune them all
echo "Pruning unused volumes..."
docker volume prune -f

echo "Restarting services..."
docker-compose up -d

echo "Prometheus and Grafana data have been reset. Wait a moment for the services to fully initialize."
echo "You can access Grafana at http://localhost:3000 with username 'admin' and password 'admin'"
echo "You can access Prometheus at http://localhost:9090"