//go:build integration
// +build integration

package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"crochet/pkg/db"
	"crochet/pkg/rabbitmq"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
)

func TestPingIntegration(t *testing.T) {
	ctx := context.Background()

	// Start PostgreSQL container
	pgC, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:14-alpine"),
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(10*time.Second),
		),
	)
	require.NoError(t, err)
	defer pgC.Terminate(ctx)

	connStr, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	dbConn, err := sql.Open("postgres", connStr)
	require.NoError(t, err)
	defer dbConn.Close()
	require.NoError(t, dbConn.Ping())

	_, err = dbConn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS items (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL
		)
	`)
	require.NoError(t, err)
	dbQueries := db.New(dbConn)

	// Start RabbitMQ container
	rabbitReq := testcontainers.ContainerRequest{
		Image:        "rabbitmq:3-management",
		ExposedPorts: []string{"5672/tcp", "15672/tcp"},
		WaitingFor:   wait.ForLog("Server startup complete"),
		Env: map[string]string{
			"RABBITMQ_DEFAULT_USER": "guest",
			"RABBITMQ_DEFAULT_PASS": "guest",
		},
	}
	rabbitC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: rabbitReq,
		Started:          true,
	})
	require.NoError(t, err)
	defer rabbitC.Terminate(ctx)

	host, err := rabbitC.Host(ctx)
	require.NoError(t, err)
	port, err := rabbitC.MappedPort(ctx, "5672")
	require.NoError(t, err)
	rmqUrl := fmt.Sprintf("amqp://guest:guest@%s:%s/", host, port.Port())
	rmqConn, err := amqp091.Dial(rmqUrl)
	require.NoError(t, err)
	defer rmqConn.Close()
	rmqQueue := rabbitmq.NewQueue(rmqConn)

	var handler *Handler
	app := fx.New(
		fx.Provide(func() rabbitmq.Queue { return rmqQueue }),
		fx.Provide(func() QueriesInterface { return dbQueries }),
		fx.Provide(NewHandler),
		fx.Populate(&handler),
	)
	require.NoError(t, app.Start(ctx))
	defer app.Stop(ctx)

	rec := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
	gctx, _ := gin.CreateTestContext(rec)
	gctx.Request = req

	// Call handler
	handler.Ping(gctx)

	// Verify HTTP response
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"message":"pong"}`, rec.Body.String())

	// Verify DB state
	item, err := dbQueries.GetItemByID(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, "exampleItem", item.Name)

	// Verify RabbitMQ queue is empty
	ch, err := rmqConn.Channel()
	require.NoError(t, err)
	defer ch.Close()
	q, err := ch.QueueDeclarePassive("ping-queue", false, false, false, false, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, q.Messages)
}
