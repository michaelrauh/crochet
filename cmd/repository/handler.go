package main

import (
	"crochet/pkg/db"
	"crochet/pkg/ortho"
	"crochet/pkg/queueenvelope"
	"crochet/pkg/redisstream"
	"log"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	queries db.QueriesInterface
	rdsq    *redisstream.Queue
}

type Corpus struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

func NewHandler(queries db.QueriesInterface, rdsq *redisstream.Queue) *Handler {
	return &Handler{
		queries: queries,
		rdsq:    rdsq,
	}
}

func RegisterRoutes(router *gin.Engine, h *Handler) {
	router.POST("/corpora", h.Corpus)
}

// handlePublishError logs the error and returns a 500 response to the client
func (h *Handler) handlePublishError(c *gin.Context, err error, operationName string) bool {
	if err != nil {
		log.Printf("Failed to publish %s: %v", operationName, err)
		c.JSON(500, gin.H{"error": "Failed to publish " + operationName})
		return true
	}
	return false
}

func (h *Handler) Corpus(c *gin.Context) {
	var corpus Corpus
	if err := c.ShouldBindJSON(&corpus); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if corpus.Title == "" || corpus.Content == "" {
		c.JSON(400, gin.H{"error": "Title and Content are required"})
		return
	}

	ctx := c.Request.Context()
	vocabulary := extractVocabulary(corpus.Content)
	subphrases := extractSubphrases(corpus.Content)

	// Publish StartSigil
	if h.handlePublishError(c, queueenvelope.PublishStartSigil(ctx, h.rdsq, "START"), "start sigil") {
		return
	}

	// Publish Vocabulary
	if h.handlePublishError(c, queueenvelope.PublishVocabulary(ctx, h.rdsq, vocabulary), "vocabulary") {
		return
	}

	// Publish Subphrases
	if h.handlePublishError(c, queueenvelope.PublishSubphrases(ctx, h.rdsq, subphrases), "subphrases") {
		return
	}

	// Publish EndSigil
	if h.handlePublishError(c, queueenvelope.PublishEndSigil(ctx, h.rdsq, "END"), "end sigil") {
		return
	}

	// Publish seed ortho
	if h.handlePublishError(c, queueenvelope.PublishOrtho(ctx, h.rdsq, ortho.NewOrtho()), "ortho") {
		return
	}

	c.JSON(202, gin.H{"message": "Corpus accepted", "title": corpus.Title})
}
