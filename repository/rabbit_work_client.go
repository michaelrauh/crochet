package main

import (
	"context"
	"crochet/httpclient"
	"crochet/types"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// RabbitWorkQueueClient wraps a RabbitClient to implement the WorkQueueClient interface
type RabbitWorkQueueClient struct {
	client *httpclient.RabbitClient[types.WorkItem]
}

// NewRabbitWorkQueueClient creates a new client for work queue operations
func NewRabbitWorkQueueClient(url string) (*RabbitWorkQueueClient, error) {
	log.Printf("DIAG: Creating new RabbitWorkQueueClient with URL: %s", url)
	client, err := httpclient.NewRabbitClient[types.WorkItem](url)
	if err != nil {
		log.Printf("DIAG: Failed to create RabbitClient for work queue: %v", err)
		return nil, err
	}
	log.Printf("DIAG: Successfully created RabbitWorkQueueClient")
	return &RabbitWorkQueueClient{
		client: client,
	}, nil
}

// PushMessage implements the RabbitQueueClient interface
func (c *RabbitWorkQueueClient) PushMessage(ctx context.Context, queueName string, message []byte) error {
	requestID := "unknown"
	if ctx.Value("request_id") != nil {
		requestID = ctx.Value("request_id").(string)
	}

	log.Printf("DIAG[%s]: Declaring queue %s before pushing message", requestID, queueName)
	if err := c.client.DeclareQueue(ctx, queueName); err != nil {
		log.Printf("DIAG[%s]: Failed to declare queue %s: %v", requestID, queueName, err)
		return err
	}

	log.Printf("DIAG[%s]: Pushing message to work queue %s (length: %d bytes)", requestID, queueName, len(message))
	// Add a snippet of the message content for debugging
	if len(message) > 0 {
		previewLength := min(50, len(message))
		log.Printf("DIAG[%s]: Message preview: %s...", requestID, string(message[:previewLength]))
	}
	return c.client.PushMessage(ctx, queueName, message)
}

// Close implements the RabbitQueueClient interface
func (c *RabbitWorkQueueClient) Close(ctx context.Context) error {
	log.Printf("DIAG: Closing RabbitWorkQueueClient")
	err := c.client.Close(ctx)
	if err != nil {
		log.Printf("DIAG: Error closing RabbitWorkQueueClient: %v", err)
	} else {
		log.Printf("DIAG: Successfully closed RabbitWorkQueueClient")
	}
	return err
}

// PopMessagesFromQueue implements the WorkQueueClient interface
func (c *RabbitWorkQueueClient) PopMessagesFromQueue(ctx context.Context, queueName string, count int) ([]RabbitMessage, error) {
	requestID := "unknown"
	if ctx.Value("request_id") != nil {
		requestID = ctx.Value("request_id").(string)
	}
	// Check RabbitMQ queue status directly before popping
	log.Printf("DIAG[%s]: Checking work queue %s status before popping messages", requestID, queueName)
	// Ensure the queue exists
	log.Printf("DIAG[%s]: Declaring queue %s before popping messages", requestID, queueName)
	if err := c.client.DeclareQueue(ctx, queueName); err != nil {
		log.Printf("DIAG[%s]: Failed to declare queue %s: %v", requestID, queueName, err)
		return nil, err
	}
	log.Printf("DIAG[%s]: Popping messages from work queue %s with count %d", requestID, queueName, count)
	messages, err := c.client.PopMessagesFromQueue(ctx, queueName, count)
	if err != nil {
		log.Printf("DIAG[%s]: Failed to pop messages from queue %s: %v", requestID, queueName, err)
		return nil, err
	}
	// Convert httpclient.MessageWithAck to our RabbitMessage
	result := make([]RabbitMessage, len(messages))
	for i, msg := range messages {
		// Fix for empty data issue - the message.Data coming from RabbitMQ is a WorkItem
		// but we need to check if it has meaningful data or if the Data field is empty

		// Log raw message data for debugging
		if msgJSON, err := json.Marshal(msg.Data); err == nil {
			log.Printf("DIAG[%s]: Raw message data: %s", requestID, string(msgJSON))
		}

		// If the message was directly pushed as an Ortho instead of a WorkItem,
		// we need to handle it specially
		if msg.Data.ID == "" && msg.Data.Data == nil {
			// This may be a raw Ortho object pushed directly by the feeder
			// We'll create a proper WorkItem wrapper for it
			log.Printf("DIAG[%s]: Detected potential raw Ortho object instead of WorkItem", requestID)

			// Create a new WorkItem with the raw message as the Data field
			workItem := types.WorkItem{
				ID:        fmt.Sprintf("work-%d", time.Now().UnixNano()),
				Data:      msg.Data, // The raw object becomes the Data field
				Timestamp: time.Now().UnixNano(),
			}

			result[i] = RabbitMessage{
				DeliveryTag: msg.DeliveryTag,
				Data:        workItem,
			}

			log.Printf("DIAG[%s]: Created WorkItem wrapper for raw message data", requestID)
		} else {
			// Normal path - the message is already a WorkItem
			result[i] = RabbitMessage{
				DeliveryTag: msg.DeliveryTag,
				Data:        msg.Data,
			}
		}

		// Log the final message that will be returned
		if resultJSON, err := json.Marshal(result[i].Data); err == nil {
			log.Printf("DIAG[%s]: Final message %d with delivery tag %d: %s",
				requestID, i, msg.DeliveryTag, string(resultJSON))
		}
	}
	log.Printf("DIAG[%s]: Successfully popped %d messages from work queue %s", requestID, len(result), queueName)
	return result, nil
}

// AckByDeliveryTag implements the WorkQueueClient interface
func (c *RabbitWorkQueueClient) AckByDeliveryTag(ctx context.Context, tag uint64) error {
	requestID := "unknown"
	if ctx.Value("request_id") != nil {
		requestID = ctx.Value("request_id").(string)
	}

	log.Printf("DIAG[%s]: Acknowledging message with delivery tag %d", requestID, tag)
	err := c.client.AckByDeliveryTag(ctx, tag)
	if err != nil {
		log.Printf("DIAG[%s]: Failed to acknowledge message with delivery tag %d: %v", requestID, tag, err)
	} else {
		log.Printf("DIAG[%s]: Successfully acknowledged message with delivery tag %d", requestID, tag)
	}
	return err
}

// min is a helper function to find the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
