package main

import (
	"context"
)

// RabbitQueueClient defines the common interface for all RabbitMQ queue clients
type RabbitQueueClient interface {
	PushMessage(ctx context.Context, queueName string, message []byte) error
	Close(ctx context.Context) error
}

// WorkQueueClient defines specialized methods for interacting with the work queue
type WorkQueueClient interface {
	RabbitQueueClient
	PopMessagesFromQueue(ctx context.Context, queueName string, count int) ([]RabbitMessage, error)
	AckByDeliveryTag(ctx context.Context, tag uint64) error
}

// DBQueueClient defines specialized methods for interacting with the DB queue
type DBQueueClient interface {
	RabbitQueueClient
}

// RabbitMessage represents a message from a RabbitMQ queue
type RabbitMessage struct {
	DeliveryTag uint64
	Data        interface{}
}
