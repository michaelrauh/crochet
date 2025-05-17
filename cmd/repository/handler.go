package main

import (
	"crochet/pkg/ortho"
	"crochet/pkg/queueenvelope"
	"crochet/pkg/rabbitmq"
	"crochet/pkg/telemetry"
	"log"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	queries QueriesInterface
	rmq     rabbitmq.Queue
}

type Corpus struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// Example curl to hit the POST /corpora endpoint:
// curl -X POST http://localhost:8080/corpora \
// -H "Content-Type: application/json" \
// -d '{"title": "Example Title", "content": "Example Content"}'

func NewHandler(queries QueriesInterface, rmq rabbitmq.Queue) *Handler {
	return &Handler{
		queries: queries,
		rmq:     rmq,
	}
}

func RegisterRoutes(router *gin.Engine, h *Handler) {
	router.GET("/ping", h.Ping)
	router.POST("/corpora", h.Corpus)
}

func (h *Handler) Corpus(c *gin.Context) {
	var corpus Corpus
	if err := c.ShouldBindJSON(&corpus); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request body"})
		return
	}
	log.Printf("Received corpus: %+v", corpus)
	vocabulary := extractVocabulary(corpus.Content)
	subphrases := extractSubphrases(corpus.Content)
	log.Printf("Extracted vocabulary: %+v", vocabulary)
	log.Printf("Extracted subphrases: %+v", subphrases)

	ctx := c.Request.Context()
	ch, err := h.rmq.CreateChannel()
	if err != nil {
		log.Printf("Failed to create RabbitMQ channel: %v", err)
		c.JSON(500, gin.H{"error": "Failed to create RabbitMQ channel"})
		return
	}
	defer ch.Close()

	// Publish StartSigil
	err = queueenvelope.PublishStartSigil(ctx, ch, "START", h.rmq.Publish)
	if err != nil {
		log.Printf("Failed to publish start sigil: %v", err)
		c.JSON(500, gin.H{"error": "Failed to publish start sigil"})
		return
	}

	// Publish Vocabulary
	err = queueenvelope.PublishVocabulary(ctx, ch, vocabulary, h.rmq.Publish)
	if err != nil {
		log.Printf("Failed to publish vocabulary: %v", err)
		c.JSON(500, gin.H{"error": "Failed to publish vocabulary"})
		return
	}

	// Publish Subphrases
	err = queueenvelope.PublishSubphrases(ctx, ch, subphrases, h.rmq.Publish)
	if err != nil {
		log.Printf("Failed to publish subphrases: %v", err)
		c.JSON(500, gin.H{"error": "Failed to publish subphrases"})
		return
	}

	// Publish EndSigil
	err = queueenvelope.PublishEndSigil(ctx, ch, "END", h.rmq.Publish)
	if err != nil {
		log.Printf("Failed to publish end sigil: %v", err)
		c.JSON(500, gin.H{"error": "Failed to publish end sigil"})
		return
	}

	// Publish Ortho
	newOrtho := ortho.NewOrtho()
	err = queueenvelope.PublishOrtho(ctx, ch, newOrtho, h.rmq.Publish)
	if err != nil {
		log.Printf("Failed to publish ortho: %v", err)
		c.JSON(500, gin.H{"error": "Failed to publish ortho"})
		return
	}

	c.JSON(202, gin.H{"message": "Received corpus"})
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
