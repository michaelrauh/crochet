#!/bin/bash

# Load .env variables
source .env

echo "Starting local PostgreSQL and RabbitMQ containers..."
docker-compose up -d

echo "Running the Go application locally..."
DATABASE_URL=$DATABASE_URL RABBITMQ_URL=$RABBITMQ_URL go run main.go
