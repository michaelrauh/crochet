package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
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

var (
	pgContainer     *postgres.PostgresContainer
	dbConn          *sql.DB
	dbQueries       *db.Queries
	rabbitContainer testcontainers.Container
	rmqConn         *amqp091.Connection
	rmqQueue        rabbitmq.Queue
	handler         *Handler
)

// TestMain sets up containers before tests and tears them down after.
func TestMain(m *testing.M) {
	ctx := context.Background()
	// Start PostgreSQL container
	pgC, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:14-alpine"),
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(5*time.Second),
		),
	)
	if err != nil {
		log.Fatalf("failed to start postgres container: %v", err)
	}
	pgContainer = pgC

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("failed to get connection string: %v", err)
	}
	dbConn, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("failed to open db connection: %v", err)
	}
	if err = dbConn.Ping(); err != nil {
		log.Fatalf("failed to ping db: %v", err)
	}

	if _, err = dbConn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS items (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL
		)
	`); err != nil {
		log.Fatalf("failed to create schema: %v", err)
	}
	dbQueries = db.New(dbConn)

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
	if err != nil {
		log.Fatalf("failed to start rabbitmq container: %v", err)
	}
	rabbitContainer = rabbitC

	host, err := rabbitContainer.Host(ctx)
	if err != nil {
		log.Fatalf("failed to get rabbit host: %v", err)
	}
	port, err := rabbitContainer.MappedPort(ctx, "5672")
	if err != nil {
		log.Fatalf("failed to get rabbit port: %v", err)
	}
	rmqUrl := fmt.Sprintf("amqp://guest:guest@%s:%s/", host, port.Port())
	rmqConn, err = amqp091.Dial(rmqUrl)
	if err != nil {
		log.Fatalf("failed to dial rabbitmq: %v", err)
	}
	rmqQueue = rabbitmq.NewQueue(rmqConn)

	// Initialize handler via fx
	app := fx.New(
		fx.Provide(func() QueriesInterface { return dbQueries }),
		fx.Provide(func() rabbitmq.Queue { return rmqQueue }),
		fx.Provide(NewHandler),
		fx.Populate(&handler),
	)
	if err = app.Start(ctx); err != nil {
		log.Fatalf("failed to start fx app: %v", err)
	}

	// Run tests
	code := m.Run()

	// Teardown
	dbConn.Close()
	pgContainer.Terminate(ctx)
	rmqConn.Close()
	rabbitContainer.Terminate(ctx)

	os.Exit(code)
}

func TestPingIntegration(t *testing.T) {
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
