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

	ch, err := h.rmq.CreateChannel()
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to create RabbitMQ channel"})
		return
	}
	defer ch.Close()

	_ = h.rmq.Publish(ctx, ch, "ping-queue", []byte("ping-message"))
	delivery, _ := h.rmq.ConsumeOne(ctx, ch, "ping-queue")
	_ = delivery.Ack(false)

	c.JSON(200, gin.H{"message": "pong"})
}
