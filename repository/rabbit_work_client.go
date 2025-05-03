package main

import (
	"context"

	"crochet/httpclient"
	"crochet/types"
)

// RabbitWorkQueueClient wraps a RabbitClient to implement the WorkQueueClient interface
type RabbitWorkQueueClient struct {
	client *httpclient.RabbitClient[types.WorkItem]
}

// NewRabbitWorkQueueClient creates a new client for work queue operations
func NewRabbitWorkQueueClient(url string) (*RabbitWorkQueueClient, error) {
	client, err := httpclient.NewRabbitClient[types.WorkItem](url)
	if err != nil {
		return nil, err
	}

	return &RabbitWorkQueueClient{
		client: client,
	}, nil
}

// PushMessage implements the RabbitQueueClient interface
func (c *RabbitWorkQueueClient) PushMessage(ctx context.Context, queueName string, message []byte) error {
	return c.client.PushMessage(ctx, queueName, message)
}

// Close implements the RabbitQueueClient interface
func (c *RabbitWorkQueueClient) Close(ctx context.Context) error {
	return c.client.Close(ctx)
}

// PopMessagesFromQueue implements the WorkQueueClient interface
func (c *RabbitWorkQueueClient) PopMessagesFromQueue(ctx context.Context, queueName string, count int) ([]RabbitMessage, error) {
	messages, err := c.client.PopMessagesFromQueue(ctx, queueName, count)
	if err != nil {
		return nil, err
	}

	// Convert httpclient.MessageWithAck to our RabbitMessage
	result := make([]RabbitMessage, len(messages))
	for i, msg := range messages {
		result[i] = RabbitMessage{
			DeliveryTag: msg.DeliveryTag,
			Data:        msg.Data,
		}
	}

	return result, nil
}

// AckByDeliveryTag implements the WorkQueueClient interface
func (c *RabbitWorkQueueClient) AckByDeliveryTag(ctx context.Context, tag uint64) error {
	return c.client.AckByDeliveryTag(ctx, tag)
}
