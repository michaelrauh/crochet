package main

import (
	"context"

	"crochet/httpclient"
	"crochet/types"
)

// RabbitDBQueueClient wraps a RabbitClient to implement the DBQueueClient interface
type RabbitDBQueueClient[T any] struct {
	client *httpclient.RabbitClient[T]
}

// NewRabbitDBQueueClient creates a new client for DB queue operations
func NewRabbitDBQueueClient[T any](url string) (*RabbitDBQueueClient[T], error) {
	client, err := httpclient.NewRabbitClient[T](url)
	if err != nil {
		return nil, err
	}

	return &RabbitDBQueueClient[T]{
		client: client,
	}, nil
}

// PushMessage implements the RabbitQueueClient interface
func (c *RabbitDBQueueClient[T]) PushMessage(ctx context.Context, queueName string, message []byte) error {
	return c.client.PushMessage(ctx, queueName, message)
}

// Close implements the RabbitQueueClient interface
func (c *RabbitDBQueueClient[T]) Close(ctx context.Context) error {
	return c.client.Close(ctx)
}

// Create specialized DB queue clients for the repository service
type OrthosDBClient = RabbitDBQueueClient[types.Ortho]
type RemediationsDBClient = RabbitDBQueueClient[types.RemediationTuple]
