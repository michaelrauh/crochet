//go:build integration
// +build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"crochet/pkg/db"
	"crochet/pkg/queueenvelope"
	"crochet/pkg/redisstream"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	rediscontainer "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
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
	err = redisClient.XGroupCreateMkStream(ctx, "db", "db", "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		require.NoError(t, err)
	}

	return redisC, redisClient, redisQueue
}

func TestCorpusIntegration(t *testing.T) {
	fmt.Println("Starting test: TestCorpusIntegration")
	ctx := context.Background()

	// Setup Redis container, client and queue
	redisC, redisClient, redisQueue := setupRedisContainer(t, ctx)
	defer func() {
		fmt.Println("Terminating Redis container")
		redisClient.Close()
		redisC.Terminate(ctx)
	}()

	var handler *Handler
	fmt.Println("Starting fx app")
	app := fx.New(
		fx.Provide(func() *redisstream.Queue { return redisQueue }),
		fx.Provide(func() db.QueriesInterface { return nil }),
		fx.Provide(NewHandler),
		fx.Populate(&handler),
	)
	require.NoError(t, app.Start(ctx))
	defer func() {
		fmt.Println("Stopping fx app")
		app.Stop(ctx)
	}()

	// Helper function to consume one message and parse its envelope
	consumeOneMessage := func(msgName string) (map[string]interface{}, map[string]interface{}) {
		fmt.Printf("Consuming %s message\n", msgName)

		// Create timeout context
		timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		// Use redisQueue.ConsumeBatch with count=1
		msgs, err := redisQueue.ConsumeBatch(timeoutCtx, "db", "db", "test-consumer", 1)
		require.NoError(t, err)
		require.NotEmpty(t, msgs)
		msg := &msgs[0]
		fmt.Printf("Received %s message with ID: %s\n", msgName, msg.ID)

		// Acknowledge the message
		err = redisQueue.Ack(ctx, "db", "db", msg.ID)
		require.NoError(t, err)

		// Parse the envelope
		envelopeStr, ok := msg.Values["envelope"].(string)
		require.True(t, ok, "Expected 'envelope' field in message values")

		var envelope map[string]interface{}
		err = json.Unmarshal([]byte(envelopeStr), &envelope)
		require.NoError(t, err)

		// Now parse the Data field which contains the actual payload
		// Convert Data to JSON bytes regardless of its actual type
		dataBytes, err := json.Marshal(envelope["Data"])
		require.NoError(t, err)

		var payload map[string]interface{}
		err = json.Unmarshal(dataBytes, &payload)
		require.NoError(t, err)

		return envelope, payload
	}

	// Set up the Corpus endpoint test
	corpus := Corpus{
		Title:   "Test Corpus",
		Content: "This is a test corpus with some words.",
	}
	jsonCorpus, err := json.Marshal(corpus)
	require.NoError(t, err)

	// Create HTTP request and context
	req, err := http.NewRequest(http.MethodPost, "/corpora", bytes.NewBuffer(jsonCorpus))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	// Call the handler
	handler.Corpus(c)

	// Verify HTTP status code is 202 (Accepted)
	assert.Equal(t, http.StatusAccepted, w.Code)

	// Verify JSON response
	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Corpus accepted", response["message"])
	assert.Equal(t, "Test Corpus", response["title"])

	// Consume all expected messages
	envelope, payload := consumeOneMessage("START")
	require.Equal(t, queueenvelope.EnvelopeTypeStartSigil, envelope["Type"])
	require.Equal(t, "START", payload["Sigil"])

	// Expect vocabulary message
	envelope, payload = consumeOneMessage("vocabulary")
	require.Equal(t, queueenvelope.EnvelopeTypeVocabulary, envelope["Type"])
	require.NotNil(t, payload["Words"])

	// Expect subphrases message
	envelope, payload = consumeOneMessage("subphrases")
	require.Equal(t, queueenvelope.EnvelopeTypeSubphrases, envelope["Type"])
	require.NotNil(t, payload["Phrases"])

	// Expect END message
	envelope, payload = consumeOneMessage("END")
	require.Equal(t, queueenvelope.EnvelopeTypeEndSigil, envelope["Type"])
	require.Equal(t, "END", payload["Sigil"])

	fmt.Println("Test completed successfully")
}
