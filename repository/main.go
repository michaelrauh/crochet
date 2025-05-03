package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"crochet/clients"
	"crochet/config"
	"crochet/health"
	"crochet/middleware"
	"crochet/types"

	"github.com/gin-gonic/gin"
	"github.com/kelseyhightower/envconfig"
)

var ctxStore types.ContextStore

func initStore(cfg config.RepositoryConfig) error {
	var err error
	connURL := cfg.ContextDBEndpoint
	ctxStore, err = types.NewLibSQLContextStore(connURL)
	if err != nil {
		return fmt.Errorf("failed to initialize context store: %w", err)
	}
	return nil
}

func main() {
	var cfg config.RepositoryConfig
	if err := envconfig.Process("REPOSITORY", &cfg); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	orthosCache, err := NewRistrettoOrthosCache()
	if err != nil {
		log.Fatalf("Failed to initialize Ristretto cache: %v", err)
	}

	router, tp, mp, pp, err := middleware.SetupCommonComponents(
		cfg.ServiceName,
		cfg.JaegerEndpoint,
		cfg.MetricsEndpoint,
		cfg.PyroscopeEndpoint,
	)
	if err != nil {
		log.Fatalf("Failed to set up application: %v", err)
	}
	defer tp.ShutdownWithTimeout(5 * time.Second)
	defer mp.ShutdownWithTimeout(5 * time.Second)
	defer pp.StopWithTimeout(5 * time.Second)

	log.Printf("HTTP client options: DialTimeout=%v, DialKeepAlive=%v, MaxIdleConns=%d, ClientTimeout=%v",
		cfg.DialTimeout, cfg.DialKeepAlive, cfg.MaxIdleConns, cfg.ClientTimeout)

	rabbitClients, err := clients.NewRabbitMQClients(cfg.RabbitMQURL, cfg.DBQueueName)
	if err != nil {
		log.Fatalf("Failed to initialize RabbitMQ clients: %v", err)
	}
	defer rabbitClients.CloseAll(context.Background())

	if err := initStore(cfg); err != nil {
		log.Fatalf("Failed to initialize context store: %v", err)
	}
	defer func() {
		if err := ctxStore.Close(); err != nil {
			log.Printf("Error closing context store: %v", err)
		}
	}()

	handler := NewRepositoryHandler(
		ctxStore,
		orthosCache,
		rabbitClients.Service,
		cfg,
	)

	router.POST("/corpora", handler.HandlePostCorpus)
	router.POST("/results", handler.HandlePostResults)
	router.GET("/context", handler.HandleGetContext)
	router.GET("/work", handler.HandleGetWork)

	healthCheck := health.New(health.Options{
		ServiceName: cfg.ServiceName,
	})
	router.GET("/health", gin.WrapF(healthCheck.Handler()))

	address := cfg.GetAddress()
	log.Printf("Server starting on %s...\n", address)
	if err := router.Run(address); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
