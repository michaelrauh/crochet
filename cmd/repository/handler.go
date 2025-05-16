package main

import (
	"crochet/pkg/rabbitmq"
	"crochet/pkg/telemetry"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	queries QueriesInterface
	rmq     rabbitmq.Queue
}

func NewHandler(queries QueriesInterface, rmq rabbitmq.Queue) *Handler {
	return &Handler{
		queries: queries,
		rmq:     rmq,
	}
}

func RegisterRoutes(router *gin.Engine, h *Handler) {
	router.GET("/ping", h.Ping)
}

func (h *Handler) Ping(c *gin.Context) {
	telemetry.IncPingCounter()
	ctx := c.Request.Context()
	h.queries.CreateItem(ctx, "exampleItem")
	h.queries.GetItemByID(ctx, 1)

	_ = h.rmq.Publish(ctx, "ping-queue", []byte("ping-message"))
	_, _ = h.rmq.ConsumeOne(ctx, "ping-queue")

	c.JSON(200, gin.H{"message": "pong"})
}
