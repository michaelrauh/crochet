// filepath: /Users/michaelrauh/dev/crochet/cmd/repository/mock_queue.go
package main

import (
	"context"
	"crochet/pkg/rabbitmq"

	"github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/mock"
)

// MockQueue is a custom mock implementation of rabbitmq.Queue interface
type MockQueue struct {
	mock.Mock
}

func NewMockQueue() *MockQueue {
	return &MockQueue{}
}

func (m *MockQueue) CreateChannel() (rabbitmq.ChannelInterface, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(rabbitmq.ChannelInterface), args.Error(1)
}

func (m *MockQueue) Publish(ctx context.Context, ch rabbitmq.ChannelInterface, queueName string, body []byte) error {
	args := m.Called(ctx, ch, queueName, body)
	return args.Error(0)
}

func (m *MockQueue) ConsumeOne(ctx context.Context, ch rabbitmq.ChannelInterface, queueName string) (*amqp091.Delivery, error) {
	args := m.Called(ctx, ch, queueName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*amqp091.Delivery), args.Error(1)
}

func (m *MockQueue) ConsumeBatch(ctx context.Context, ch rabbitmq.ChannelInterface, queueName string, max int) ([]amqp091.Delivery, error) {
	args := m.Called(ctx, ch, queueName, max)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]amqp091.Delivery), args.Error(1)
}
