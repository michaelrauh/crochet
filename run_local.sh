#!/bin/bash

# Load .env variables
source .env

echo "Starting local PostgreSQL and RabbitMQ containers..."
docker-compose up -d

echo "Building the Rust application..."
cargo build --release

echo "Running the Rust application locally..."
DATABASE_URL=$DATABASE_URL RABBITMQ_URL=$RABBITMQ_URL ./target/release/crochet
