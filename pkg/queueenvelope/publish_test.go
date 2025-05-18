//go:build integration
// +build integration

package queueenvelope

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	rediscontainer "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"

	"crochet/pkg/ortho"
	"crochet/pkg/redisstream"
)

// setupRedisContainer initializes a Redis container for testing
func setupRedisContainer(t *testing.T, ctx context.Context) (*rediscontainer.RedisContainer, *redis.Client, *redisstream.Queue) {
	// Start Redis container
	redisC, err := rediscontainer.RunContainer(ctx,
		testcontainers.WithImage("redis:7.2"),
		rediscontainer.WithSnapshotting(1, 1),
		testcontainers.WithWaitStrategy(
			wait.ForLog("Ready to accept connections").WithStartupTimeout(10*time.Second),
		),
	)
	require.NoError(t, err)

	// Get Redis connection details
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

	// Create Redis Stream queue
	redisQueue := redisstream.NewQueue(redisClient)

	// Initialize stream and consumer group
	err = redisClient.XGroupCreateMkStream(ctx, "db", "test-group", "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		require.NoError(t, err)
	}

	return redisC, redisClient, redisQueue
}

// TestPublishVocabularyIntegration tests the PublishVocabulary method
func TestPublishVocabularyIntegration(t *testing.T) {
	ctx := context.Background()
	redisC, redisClient, redisQueue := setupRedisContainer(t, ctx)
	defer func() {
		redisClient.Close()
		if err := redisC.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate Redis container: %v", err)
		}
	}()

	// Test PublishVocabulary
	words := []string{"hello", "world"}
	err := PublishVocabulary(ctx, redisQueue, words)
	require.NoError(t, err)

	// Read the message
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	msgs, err := redisQueue.ConsumeBatch(timeoutCtx, "db", "test-group", "test-consumer", 1)
	require.NoError(t, err)
	require.NotEmpty(t, msgs)
	msg := &msgs[0]

	// Verify message content
	var env Envelope
	err = json.Unmarshal([]byte(msg.Values["envelope"].(string)), &env)
	require.NoError(t, err)
	assert.Equal(t, EnvelopeTypeVocabulary, env.Type)

	var payload VocabularyPayload
	err = json.Unmarshal(env.Data, &payload)
	require.NoError(t, err)
	assert.ElementsMatch(t, words, payload.Words)
}

// TestPublishSubphrasesIntegration tests the PublishSubphrases method
func TestPublishSubphrasesIntegration(t *testing.T) {
	ctx := context.Background()
	redisC, redisClient, redisQueue := setupRedisContainer(t, ctx)
	defer func() {
		redisClient.Close()
		if err := redisC.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate Redis container: %v", err)
		}
	}()

	// Test PublishSubphrases
	phrases := [][]string{{"test", "phrases"}, {"more", "phrases"}}
	err := PublishSubphrases(ctx, redisQueue, phrases)
	require.NoError(t, err)

	// Read the message
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	msgs, err := redisQueue.ConsumeBatch(timeoutCtx, "db", "test-group", "test-consumer", 1)
	require.NoError(t, err)
	require.NotEmpty(t, msgs)
	msg := &msgs[0]

	// Verify message content
	var env Envelope
	err = json.Unmarshal([]byte(msg.Values["envelope"].(string)), &env)
	require.NoError(t, err)
	assert.Equal(t, EnvelopeTypeSubphrases, env.Type)

	var payload SubphrasesPayload
	err = json.Unmarshal(env.Data, &payload)
	require.NoError(t, err)
	assert.Equal(t, phrases, payload.Phrases)
}

// TestPublishStartSigilIntegration tests the PublishStartSigil method
func TestPublishStartSigilIntegration(t *testing.T) {
	ctx := context.Background()
	redisC, redisClient, redisQueue := setupRedisContainer(t, ctx)
	defer func() {
		redisClient.Close()
		if err := redisC.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate Redis container: %v", err)
		}
	}()

	// Test PublishStartSigil
	sigil := "START"
	err := PublishStartSigil(ctx, redisQueue, sigil)
	require.NoError(t, err)

	// Read the message
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	msgs, err := redisQueue.ConsumeBatch(timeoutCtx, "db", "test-group", "test-consumer", 1)
	require.NoError(t, err)
	require.NotEmpty(t, msgs)
	msg := &msgs[0]

	// Verify message content
	var env Envelope
	err = json.Unmarshal([]byte(msg.Values["envelope"].(string)), &env)
	require.NoError(t, err)
	assert.Equal(t, EnvelopeTypeStartSigil, env.Type)

	var payload StartSigilPayload
	err = json.Unmarshal(env.Data, &payload)
	require.NoError(t, err)
	assert.Equal(t, sigil, payload.Sigil)
}

// TestPublishEndSigilIntegration tests the PublishEndSigil method
func TestPublishEndSigilIntegration(t *testing.T) {
	ctx := context.Background()
	redisC, redisClient, redisQueue := setupRedisContainer(t, ctx)
	defer func() {
		redisClient.Close()
		if err := redisC.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate Redis container: %v", err)
		}
	}()

	// Test PublishEndSigil
	sigil := "END"
	err := PublishEndSigil(ctx, redisQueue, sigil)
	require.NoError(t, err)

	// Read the message
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	msgs, err := redisQueue.ConsumeBatch(timeoutCtx, "db", "test-group", "test-consumer", 1)
	require.NoError(t, err)
	require.NotEmpty(t, msgs)
	msg := &msgs[0]

	// Verify message content
	var env Envelope
	err = json.Unmarshal([]byte(msg.Values["envelope"].(string)), &env)
	require.NoError(t, err)
	assert.Equal(t, EnvelopeTypeEndSigil, env.Type)

	var payload EndSigilPayload
	err = json.Unmarshal(env.Data, &payload)
	require.NoError(t, err)
	assert.Equal(t, sigil, payload.Sigil)
}

// TestPublishOrthoIntegration tests the PublishOrtho method
func TestPublishOrthoIntegration(t *testing.T) {
	ctx := context.Background()
	redisC, redisClient, redisQueue := setupRedisContainer(t, ctx)
	defer func() {
		redisClient.Close()
		if err := redisC.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate Redis container: %v", err)
		}
	}()

	// Test PublishOrtho
	o := ortho.NewOrtho()
	// Set test data on the remaining fields
	o.Grid = []interface{}{"test-data"}

	err := PublishOrtho(ctx, redisQueue, o)
	require.NoError(t, err)

	// Read the message
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	msgs, err := redisQueue.ConsumeBatch(timeoutCtx, "db", "test-group", "test-consumer", 1)
	require.NoError(t, err)
	require.NotEmpty(t, msgs)
	msg := &msgs[0]

	// Verify message content
	var env Envelope
	err = json.Unmarshal([]byte(msg.Values["envelope"].(string)), &env)
	require.NoError(t, err)
	assert.Equal(t, EnvelopeTypeOrtho, env.Type)

	// We don't verify the full ortho content as it would be complex,
	// but we ensure the envelope type is correct
}
