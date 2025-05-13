package main

import (
	"crochet/pkg/telemetry"

	"github.com/gin-gonic/gin"
)

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

func RegisterRoutes(router *gin.Engine, h *Handler) {
	router.GET("/ping", h.Ping)
}

func (h *Handler) Ping(c *gin.Context) {
	telemetry.IncPingCounter()

	c.JSON(200, gin.H{"message": "pong"})
}
