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

	"crochet/pkg/rabbitmq"

	"github.com/gin-gonic/gin"
	"github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
)

func TestCorpusIntegration(t *testing.T) {
	fmt.Println("Starting test: TestCorpusIntegration")
	ctx := context.Background()

	rabbitReq := testcontainers.ContainerRequest{
		Image:        "rabbitmq:3-management",
		ExposedPorts: []string{"5672/tcp", "15672/tcp"},
		WaitingFor:   wait.ForLog("Server startup complete"),
		Env: map[string]string{
			"RABBITMQ_DEFAULT_USER": "guest",
			"RABBITMQ_DEFAULT_PASS": "guest",
		},
	}
	fmt.Println("Starting RabbitMQ container")
	rabbitC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: rabbitReq,
		Started:          true,
	})
	require.NoError(t, err)
	defer func() {
		fmt.Println("Terminating RabbitMQ container")
		rabbitC.Terminate(ctx)
	}()

	fmt.Println("Getting RabbitMQ host and port")
	host, err := rabbitC.Host(ctx)
	require.NoError(t, err)
	port, err := rabbitC.MappedPort(ctx, "5672")
	require.NoError(t, err)
	rmqUrl := fmt.Sprintf("amqp://guest:guest@%s:%s/", host, port.Port())
	fmt.Printf("Connecting to RabbitMQ at %s\n", rmqUrl)
	rmqConn, err := amqp091.Dial(rmqUrl)
	require.NoError(t, err)
	defer func() {
		fmt.Println("Closing RabbitMQ connection")
		rmqConn.Close()
	}()
	rmqQueue := rabbitmq.NewQueue(rmqConn)

	var handler *Handler
	fmt.Println("Starting fx app")
	app := fx.New(
		fx.Provide(func() rabbitmq.Queue { return rmqQueue }),
		fx.Provide(func() QueriesInterface { return nil }),
		fx.Provide(NewHandler),
		fx.Populate(&handler),
	)
	require.NoError(t, app.Start(ctx))
	defer func() {
		fmt.Println("Stopping fx app")
		app.Stop(ctx)
	}()

	fmt.Println("Opening RabbitMQ channel")
	ch, err := rmqConn.Channel()
	require.NoError(t, err)
	defer func() {
		fmt.Println("Closing RabbitMQ channel")
		ch.Close()
	}()

	fmt.Println("Declaring db queue")
	_, err = ch.QueueDeclare("db", false, false, false, false, nil)
	require.NoError(t, err)

	fmt.Println("Preparing HTTP request for handler")
	rec := httptest.NewRecorder()
	body := map[string]string{
		"title":   "Example Title",
		"content": "Example Content",
	}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, "/corpora", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	gctx, _ := gin.CreateTestContext(rec)
	gctx.Request = req

	fmt.Println("Calling handler.Corpus")
	handler.Corpus(gctx)
	fmt.Println("handler.Corpus returned")

	fmt.Printf("HTTP response code: %d, body: %s\n", rec.Code, rec.Body.String())
	assert.Equal(t, http.StatusAccepted, rec.Code)
	assert.JSONEq(t, `{"message":"Received corpus"}`, rec.Body.String())

	// Helper function to consume one message with a fresh channel
	consumeOneMessage := func(msgName string) []byte {
		fmt.Printf("Consuming %s message\n", msgName)

		// Create a fresh channel for each consumption
		freshCh, err := rmqConn.Channel()
		require.NoError(t, err)
		defer freshCh.Close()

		// Create timeout context
		timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		// Use rmqQueue.ConsumeOne with the fresh channel
		msg, err := rmqQueue.ConsumeOne(timeoutCtx, freshCh, "db")
		require.NoError(t, err)

		fmt.Printf("Received %s message: %s\n", msgName, string(msg.Body))
		msg.Ack(false)
		return msg.Body
	}

	// Consume all expected messages
	startMsgBody := consumeOneMessage("START")
	require.Contains(t, string(startMsgBody), "START")

	vocabMsgBody := consumeOneMessage("VOCABULARY")
	require.NotEmpty(t, vocabMsgBody)

	subphrasesMsgBody := consumeOneMessage("SUBPHRASES")
	require.NotEmpty(t, subphrasesMsgBody)

	endMsgBody := consumeOneMessage("END")
	require.Contains(t, string(endMsgBody), "END")

	orthoMsgBody := consumeOneMessage("ORTHO")
	require.NotEmpty(t, orthoMsgBody)

	fmt.Println("TestCorpusIntegration completed successfully")
}
