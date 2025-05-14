//go:build !e2e
// +build !e2e

package main

import (
	"context"
	"crochet/pkg/db"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
)

var (
	pgContainer *postgres.PostgresContainer
	dbConn      *sql.DB
	dbQueries   *db.Queries
	handler     *Handler
)

// Sets up a real PostgreSQL container for integration testing
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

	// Real dependency injection with fx
	app := fx.New(
		fx.Provide(func() QueriesInterface { return dbQueries }), // Provide a real db.Queries as QueriesInterface
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

	It("should store items in the database when calling Ping", func() {
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

		// List all created items to verify
		rows, err := dbConn.QueryContext(dbCtx, "SELECT id, name FROM items")
		Expect(err).NotTo(HaveOccurred())
		defer rows.Close()

		var items []db.Item
		for rows.Next() {
			var item db.Item
			err := rows.Scan(&item.ID, &item.Name)
			Expect(err).NotTo(HaveOccurred())
			items = append(items, item)
		}
		Expect(rows.Err()).NotTo(HaveOccurred())

		// We should have at least one item
		Expect(len(items)).To(BeNumerically(">=", 1))

		// The first item (or one of them) should be our example item
		found := false
		for _, item := range items {
			if item.Name == "exampleItem" {
				found = true
				break
			}
		}
		Expect(found).To(BeTrue(), "Expected to find 'exampleItem' in the database")
	})
})
