package main

import (
	"crochet/telemetry"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// handleRoot handles the root endpoint
func handleRoot(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Workserver API",
		"version": "1.0.0",
	})
}

// handlePush handles the push endpoint to add orthos to the work queue
func handlePush(c *gin.Context) {
	var request PushRequest

	// Bind JSON to struct
	if err := c.ShouldBindJSON(&request); err != nil {
		telemetry.LogAndError(c, err, "workserver", "Error parsing push request")
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request format",
		})
		return
	}

	log.Printf("Received push request with %d orthos", len(request.Orthos))

	// Push orthos to the work queue
	ids := workQueue.Push(request.Orthos)

	c.JSON(http.StatusOK, PushResponse{
		Status:  "success",
		Message: "Items pushed to work queue successfully",
		Count:   len(request.Orthos),
		IDs:     ids,
	})
}

// handlePop handles the pop endpoint to retrieve items from the work queue
func handlePop(c *gin.Context) {
	// Pop an item from the queue
	item, found := workQueue.Pop()

	if !found {
		// No items in queue
		c.JSON(http.StatusOK, PopResponse{
			Status:  "success",
			Message: "No items available in work queue",
			Ortho:   nil,
			ID:      "",
		})
		return
	}

	c.JSON(http.StatusOK, PopResponse{
		Status:  "success",
		Message: "Item popped from work queue successfully",
		Ortho:   &item.Ortho,
		ID:      item.ID,
	})
}

// handleAck handles the ack endpoint to acknowledge completed work items
func handleAck(c *gin.Context) {
	var request AckRequest

	// Bind JSON to struct
	if err := c.ShouldBindJSON(&request); err != nil {
		telemetry.LogAndError(c, err, "workserver", "Error parsing ack request")
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request format",
		})
		return
	}

	// Acknowledge the work item
	success := workQueue.Ack(request.ID)

	if !success {
		c.JSON(http.StatusNotFound, AckResponse{
			Status:  "error",
			Message: "Work item not found or not in progress",
		})
		return
	}

	c.JSON(http.StatusOK, AckResponse{
		Status:  "success",
		Message: "Work item acknowledged successfully",
	})
}

// handleNack handles the nack endpoint to return work items to the queue
func handleNack(c *gin.Context) {
	var request AckRequest

	// Bind JSON to struct (reusing AckRequest since the structure is the same)
	if err := c.ShouldBindJSON(&request); err != nil {
		telemetry.LogAndError(c, err, "workserver", "Error parsing nack request")
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request format",
		})
		return
	}

	// Negative acknowledge the work item
	success := workQueue.Nack(request.ID)

	if !success {
		c.JSON(http.StatusNotFound, AckResponse{
			Status:  "error",
			Message: "Work item not found or not in progress",
		})
		return
	}

	c.JSON(http.StatusOK, AckResponse{
		Status:  "success",
		Message: "Work item returned to queue successfully",
	})
}
