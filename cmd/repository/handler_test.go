package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"crochet/mocks"
	"crochet/pkg/db"

	"github.com/gin-gonic/gin"
	"github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockChannel implements rabbitmq.ChannelInterface for testing
type MockChannel struct {
	mock.Mock
}

func (m *MockChannel) Close() error {
	args := m.Called()
	return args.Error(0)
}

// Add stubs to satisfy pkg/rabbitmq.ChannelInterface
func (m *MockChannel) Qos(prefetchCount, prefetchSize int, global bool) error {
	// stubbed no-op
	return nil
}

func (m *MockChannel) Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp091.Table) (<-chan amqp091.Delivery, error) {
	// stubbed no-op
	return make(chan amqp091.Delivery), nil
}

// MockAcknowledger implements amqp091.Acknowledger for testing
type MockAcknowledger struct {
	mock.Mock
}

func (m *MockAcknowledger) Ack(tag uint64, multiple bool) error {
	args := m.Called(tag, multiple)
	return args.Error(0)
}

func (m *MockAcknowledger) Nack(tag uint64, multiple bool, requeue bool) error {
	args := m.Called(tag, multiple, requeue)
	return args.Error(0)
}

func (m *MockAcknowledger) Reject(tag uint64, requeue bool) error {
	args := m.Called(tag, requeue)
	return args.Error(0)
}

func TestHandler_Ping(t *testing.T) {
	// Set up mocks for database queries and RabbitMQ queue
	queries := mocks.NewQueriesInterface(t)
	queue := NewMockQueue()
	chMock := new(MockChannel)
	ack := new(MockAcknowledger)

	// Prepare a delivery with our acknowledger
	delivery := &amqp091.Delivery{Acknowledger: ack}

	// Expectations: database interactions
	ctx := context.Background()
	queries.On("CreateItem", ctx, "exampleItem").Return(db.Item{}, nil)
	queries.On("GetItemByID", ctx, int32(1)).Return(db.Item{}, nil)

	// Expectations: RabbitMQ interactions
	queue.On("CreateChannel").Return(chMock, nil)
	queue.On("Publish", ctx, chMock, "ping-queue", []byte("ping-message")).Return(nil)
	queue.On("ConsumeOne", ctx, chMock, "ping-queue").Return(delivery, nil)

	// Channel close and ack should be called once without error
	chMock.On("Close").Return(nil)
	ack.On("Ack", mock.Anything, false).Return(nil)

	// Create handler and request context
	h := NewHandler(queries, queue)
	rec := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
	gctx, _ := gin.CreateTestContext(rec)
	gctx.Request = req

	// Call the handler
	h.Ping(gctx)

	// Verify HTTP response
	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "pong", resp["message"])

	// Assert all mock expectations
	queries.AssertExpectations(t)
	queue.AssertExpectations(t)
	chMock.AssertExpectations(t)
	ack.AssertExpectations(t)
}
