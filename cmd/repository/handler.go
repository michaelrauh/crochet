package main

import (
	"crochet/pkg/telemetry"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	queries QueriesInterface
}

func NewHandler(queries QueriesInterface) *Handler {
	return &Handler{
		queries: queries,
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
	c.JSON(200, gin.H{"message": "pong"})
}
