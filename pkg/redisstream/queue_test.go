//go:build integration
// +build integration

package redisstream

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	rediscontainer "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

// setupRedisContainer initializes a Redis container for testing
func setupRedisContainer(t *testing.T, ctx context.Context) (*rediscontainer.RedisContainer, *redis.Client) {
	// Start Redis container
	redisC, err := rediscontainer.RunContainer(ctx,
		testcontainers.WithImage("redis:7.2"),
		rediscontainer.WithSnapshotting(1, 1),
		testcontainers.WithWaitStrategy(
			wait.ForLog("Ready to accept connections").WithStartupTimeout(10*time.Second),
		),
	)
	require.NoError(t, err)

	// Get connection details
	redisHost, err := redisC.Host(ctx)
	require.NoError(t, err)
	redisPort, err := redisC.MappedPort(ctx, "6379")
	require.NoError(t, err)

	// Create Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", redisHost, redisPort.Port()),
	})

	// Enable AOF persistence
	result := redisClient.ConfigSet(ctx, "appendonly", "yes")
	require.NoError(t, result.Err())

	return redisC, redisClient
}

func TestQueue_PushAndConsumeOneIntegration(t *testing.T) {
	ctx := context.Background()

	// Setup Redis container and client
	redisC, redisClient := setupRedisContainer(t, ctx)
	defer func() {
		redisClient.Close()
		if err := redisC.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate Redis container: %v", err)
		}
	}()

	// Create queue
	q := NewQueue(redisClient)

	// Create stream and consumer group
	streamName := "test-stream"
	groupName := "test-group"
	consumerName := "test-consumer"

	err := redisClient.XGroupCreateMkStream(ctx, streamName, groupName, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		require.NoError(t, err)
	}

	// Test publishing a message using PublishBatch instead of Publish
	values := map[string]interface{}{"foo": "bar"}
	err = q.PublishBatch(ctx, streamName, []map[string]interface{}{values})
	require.NoError(t, err)

	// Test consuming a message
	msgs, err := q.ConsumeBatch(ctx, streamName, groupName, consumerName, 1)
	require.NoError(t, err)
	require.NotEmpty(t, msgs)
	msg := &msgs[0]
	assert.Equal(t, values["foo"], msg.Values["foo"])

	// Acknowledge the message using the queue's Ack method
	err = q.Ack(ctx, streamName, groupName, msg.ID)
	require.NoError(t, err)
}

func TestQueue_PublishMultipleAndConsumeBatchIntegration(t *testing.T) {
	ctx := context.Background()

	// Setup Redis container and client
	redisC, redisClient := setupRedisContainer(t, ctx)
	defer func() {
		redisClient.Close()
		if err := redisC.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate Redis container: %v", err)
		}
	}()

	// Create queue
	q := NewQueue(redisClient)

	// Create stream and consumer group
	streamName := "test-stream-batch"
	groupName := "test-group-batch"
	consumerName := "test-consumer-batch"

	err := redisClient.XGroupCreateMkStream(ctx, streamName, groupName, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		require.NoError(t, err)
	}

	// Test publishing multiple messages using PublishBatch instead of individual Publish calls
	values1 := map[string]interface{}{"foo": "bar"}
	values2 := map[string]interface{}{"baz": "qux"}
	values3 := map[string]interface{}{"hello": "world"}

	// Publish the first message
	err = q.PublishBatch(ctx, streamName, []map[string]interface{}{values1})
	require.NoError(t, err)

	// Publish the second message
	err = q.PublishBatch(ctx, streamName, []map[string]interface{}{values2})
	require.NoError(t, err)

	// Publish the third message
	err = q.PublishBatch(ctx, streamName, []map[string]interface{}{values3})
	require.NoError(t, err)

	// Test consuming a batch of messages
	batchSize := 2
	msgs, err := q.ConsumeBatch(ctx, streamName, groupName, consumerName, batchSize)
	require.NoError(t, err)
	require.NotNil(t, msgs)
	require.Len(t, msgs, batchSize)

	// Verify first two messages
	assert.Equal(t, values1["foo"], msgs[0].Values["foo"])
	assert.Equal(t, values2["baz"], msgs[1].Values["baz"])

	// Acknowledge the batch using the queue's Ack method
	var ids []string
	for _, msg := range msgs {
		ids = append(ids, msg.ID)
	}

	err = q.Ack(ctx, streamName, groupName, ids...)
	require.NoError(t, err)

	// Test consuming remaining message
	msgs, err = q.ConsumeBatch(ctx, streamName, groupName, consumerName, batchSize)
	require.NoError(t, err)
	require.NotNil(t, msgs)
	require.Len(t, msgs, 1)

	// Verify the third message
	assert.Equal(t, values3["hello"], msgs[0].Values["hello"])
}
