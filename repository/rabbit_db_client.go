package main

import (
	"context"
	"crochet/httpclient"
	"crochet/types"
	"log"
)

// RabbitDBQueueClient wraps a RabbitClient to implement the DBQueueClient interface
type RabbitDBQueueClient[T any] struct {
	client *httpclient.RabbitClient[T]
}

// NewRabbitDBQueueClient creates a new client for DB queue operations
func NewRabbitDBQueueClient[T any](url string) (*RabbitDBQueueClient[T], error) {
	log.Printf("DIAG: Creating new RabbitDBQueueClient with URL: %s", url)
	client, err := httpclient.NewRabbitClient[T](url)
	if err != nil {
		log.Printf("DIAG: Failed to create RabbitClient: %v", err)
		return nil, err
	}
	log.Printf("DIAG: Successfully created RabbitDBQueueClient")
	return &RabbitDBQueueClient[T]{
		client: client,
	}, nil
}

// PushMessage implements the RabbitQueueClient interface
func (c *RabbitDBQueueClient[T]) PushMessage(ctx context.Context, queueName string, message []byte) error {
	requestID := "unknown"
	if ctx.Value("request_id") != nil {
		requestID = ctx.Value("request_id").(string)
	}

	log.Printf("DIAG[%s]: RabbitDBQueueClient pushing message to queue %s, message size: %d bytes",
		requestID, queueName, len(message))

	// First ensure the queue exists
	log.Printf("DIAG[%s]: Declaring queue %s before pushing message", requestID, queueName)
	err := c.client.DeclareQueue(ctx, queueName)
	if err != nil {
		log.Printf("DIAG[%s]: Failed to declare queue %s: %v", requestID, queueName, err)
		return err
	}

	err = c.client.PushMessage(ctx, queueName, message)
	if err != nil {
		log.Printf("DIAG[%s]: Failed to push message to queue %s: %v", requestID, queueName, err)
		return err
	}

	log.Printf("DIAG[%s]: Successfully pushed message to queue %s", requestID, queueName)
	return nil
}

// Close implements the RabbitQueueClient interface
func (c *RabbitDBQueueClient[T]) Close(ctx context.Context) error {
	log.Printf("DIAG: Closing RabbitDBQueueClient")
	err := c.client.Close(ctx)
	if err != nil {
		log.Printf("DIAG: Error closing RabbitDBQueueClient: %v", err)
		return err
	}
	log.Printf("DIAG: Successfully closed RabbitDBQueueClient")
	return nil
}

// Create specialized DB queue clients for the repository service
type OrthosDBClient = RabbitDBQueueClient[types.Ortho]
type RemediationsDBClient = RabbitDBQueueClient[types.RemediationTuple]
