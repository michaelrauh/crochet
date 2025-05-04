package clients

import (
	"context"
	"fmt"

	"crochet/httpclient"
	"crochet/types"
)

// RabbitMQClients holds a single RabbitMQ client for the DB queue
type RabbitMQClients struct {
	DBQueueClient *httpclient.RabbitClient[types.DBQueueItem]
	Service       types.RabbitMQService
}

// NewRabbitMQClients initializes a RabbitMQ client for the DB queue
// and returns it along with the RabbitMQService that uses it
func NewRabbitMQClients(url string, queueName string) (*RabbitMQClients, error) {
	// Create DB queue client
	dbQueueClient, err := httpclient.NewRabbitClient[types.DBQueueItem](url)
	if err != nil {
		return nil, fmt.Errorf("failed to create DB queue client: %w", err)
	}

	// Create RabbitMQ service
	service, err := NewRabbitMQService(url, queueName)
	if err != nil {
		dbQueueClient.Close(context.Background())
		return nil, fmt.Errorf("failed to create RabbitMQ service: %w", err)
	}

	return &RabbitMQClients{
		DBQueueClient: dbQueueClient,
		Service:       service,
	}, nil
}

// CloseAll cleanly closes all RabbitMQ clients
func (rc *RabbitMQClients) CloseAll(ctx context.Context) {
	if rc.DBQueueClient != nil {
		rc.DBQueueClient.Close(ctx)
	}
}
