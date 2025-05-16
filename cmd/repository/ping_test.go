//go:build !e2e
// +build !e2e

package main

import (
	"context"
	"crochet/pkg/db"
	"crochet/pkg/rabbitmq"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rabbitmq/amqp091-go"
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

// Sets up a real PostgreSQL container and RabbitMQ container for integration testing
var _ = BeforeSuite(func() {
	ctx := context.Background()

	// Create PostgreSQL container
	container, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:14-alpine"),
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(5*time.Second),
		),
	)
	Expect(err).NotTo(HaveOccurred())
	pgContainer = container

	// Get connection details
	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	Expect(err).NotTo(HaveOccurred())

	// Connect to the database
	dbConn, err = sql.Open("postgres", connStr)
	Expect(err).NotTo(HaveOccurred())
	Expect(dbConn.Ping()).To(Succeed())

	// Create the schema
	_, err = dbConn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS items (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL
		)
	`)
	Expect(err).NotTo(HaveOccurred())

	// Initialize db.Queries
	dbQueries = db.New(dbConn)

	// Start RabbitMQ container using generic testcontainers-go API
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
	Expect(err).NotTo(HaveOccurred())
	rabbitContainer = rabbitC

	host, err := rabbitC.Host(ctx)
	Expect(err).NotTo(HaveOccurred())
	port, err := rabbitC.MappedPort(ctx, "5672")
	Expect(err).NotTo(HaveOccurred())
	rmqUrl := fmt.Sprintf("amqp://guest:guest@%s:%s/", host, port.Port())

	rmqConn, err = amqp091.Dial(rmqUrl)
	Expect(err).NotTo(HaveOccurred())
	rmqQueue = rabbitmq.NewQueue(rmqConn)

	// Real dependency injection with fx
	app := fx.New(
		fx.Provide(func() QueriesInterface { return dbQueries }), // Provide a real db.Queries as QueriesInterface
		fx.Provide(func() rabbitmq.Queue { return rmqQueue }),    // Provide a real RabbitMQ queue
		fx.Provide(NewHandler),
		fx.Populate(&handler),
	)
	Expect(app.Start(context.Background())).To(Succeed())
})

var _ = AfterSuite(func() {
	ctx := context.Background()
	if dbConn != nil {
		Expect(dbConn.Close()).To(Succeed())
	}
	if pgContainer != nil {
		Expect(pgContainer.Terminate(ctx)).To(Succeed())
	}
	if rmqConn != nil {
		Expect(rmqConn.Close()).To(Succeed())
	}
	if rabbitContainer != nil {
		Expect(rabbitContainer.Terminate(ctx)).To(Succeed())
	}
})

var _ = Describe("Repository Handler Integration", func() {
	var (
		recorder *httptest.ResponseRecorder
		ctx      *gin.Context
	)

	BeforeEach(func() {
		recorder = httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ping", nil)
		ctx, _ = gin.CreateTestContext(recorder)
		ctx.Request = req
	})

	It("should store items in the database and interact with RabbitMQ when calling Ping", func() {
		// Call the Ping method
		handler.Ping(ctx)

		// Verify HTTP response
		Expect(recorder.Code).To(Equal(http.StatusOK))
		Expect(recorder.Body.String()).To(MatchJSON(`{"message":"pong"}`))

		// Verify data was actually written to the database (integration test)
		dbCtx := context.Background()
		item, err := dbQueries.GetItemByID(dbCtx, 1)
		Expect(err).NotTo(HaveOccurred())
		Expect(item.Name).To(Equal("exampleItem"))

		// Verify RabbitMQ queue has been used (queue should be empty after consume)
		ch, err := rmqConn.Channel()
		Expect(err).NotTo(HaveOccurred())
		q, err := ch.QueueDeclarePassive("ping-queue", false, false, false, false, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(q.Messages).To(Equal(0))
		_ = ch.Close()
	})
})
