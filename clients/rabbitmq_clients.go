package clients

import (
	"context"
	"fmt"

	"crochet/httpclient"
	"crochet/types"
)

// RabbitMQClients holds all RabbitMQ clients needed by services
type RabbitMQClients struct {
	ContextClient *httpclient.RabbitClient[types.ContextInput]
	VersionClient *httpclient.RabbitClient[types.VersionInfo]
	PairsClient   *httpclient.RabbitClient[types.Pair]
	SeedClient    *httpclient.RabbitClient[types.Ortho]
	Service       types.RabbitMQService
}

// NewRabbitMQClients initializes all RabbitMQ clients and returns them as a struct
// along with the RabbitMQService that uses them
func NewRabbitMQClients(url string, queueName string) (*RabbitMQClients, error) {
	// Context RabbitMQ client
	contextClient, err := httpclient.NewRabbitClient[types.ContextInput](url)
	if err != nil {
		return nil, fmt.Errorf("failed to create context RabbitMQ client: %w", err)
	}

	// Version RabbitMQ client
	versionClient, err := httpclient.NewRabbitClient[types.VersionInfo](url)
	if err != nil {
		contextClient.Close(context.Background())
		return nil, fmt.Errorf("failed to create version RabbitMQ client: %w", err)
	}

	// Pairs RabbitMQ client
	pairsClient, err := httpclient.NewRabbitClient[types.Pair](url)
	if err != nil {
		contextClient.Close(context.Background())
		versionClient.Close(context.Background())
		return nil, fmt.Errorf("failed to create pairs RabbitMQ client: %w", err)
	}

	// Seed RabbitMQ client
	seedClient, err := httpclient.NewRabbitClient[types.Ortho](url)
	if err != nil {
		contextClient.Close(context.Background())
		versionClient.Close(context.Background())
		pairsClient.Close(context.Background())
		return nil, fmt.Errorf("failed to create seed RabbitMQ client: %w", err)
	}

	// Create RabbitMQ service
	service := NewRabbitMQService(
		url,
		queueName,
		contextClient,
		versionClient,
		pairsClient,
		seedClient,
	)

	return &RabbitMQClients{
		ContextClient: contextClient,
		VersionClient: versionClient,
		PairsClient:   pairsClient,
		SeedClient:    seedClient,
		Service:       service,
	}, nil
}

// CloseAll cleanly closes all RabbitMQ clients
func (rc *RabbitMQClients) CloseAll(ctx context.Context) {
	if rc.ContextClient != nil {
		rc.ContextClient.Close(ctx)
	}
	if rc.VersionClient != nil {
		rc.VersionClient.Close(ctx)
	}
	if rc.PairsClient != nil {
		rc.PairsClient.Close(ctx)
	}
	if rc.SeedClient != nil {
		rc.SeedClient.Close(ctx)
	}
}
