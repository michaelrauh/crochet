package main

import (
	"database/sql"
	"fmt"
	"os"

	pq "github.com/lib/pq"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	rabbitmqURL := os.Getenv("RABBITMQ_URL")

	fmt.Printf("DATABASE_URL: %s\n", databaseURL)
	fmt.Printf("RABBITMQ_URL: %s\n", rabbitmqURL)

	// Connect to the database
	connStr, err := pq.ParseURL(databaseURL)
	if err != nil {
		panic(fmt.Sprintf("Error parsing database URL: %s", err))
	}
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		panic(fmt.Sprintf("Error connecting to database: %s", err))
	}
	defer db.Close()
	fmt.Println("Connected to database")

	// Connect to RabbitMQ
	conn, err := amqp.Dial(rabbitmqURL)
	if err != nil {
		panic(fmt.Sprintf("Error connecting to RabbitMQ: %s", err))
	}
	defer conn.Close()
	fmt.Println("Connected to queue")

	// ...existing code...
}
