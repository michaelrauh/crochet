package main

import (
	"crochet/telemetry"
	"fmt"
	"log"
	"net/http"
	"time"

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

	// Validate ID
	if request.ID == "" {
		log.Printf("Received ACK request with empty ID")
		c.JSON(http.StatusBadRequest, AckResponse{
			Status:  "error",
			Message: "Empty work item ID is not valid",
		})
		return
	}

	// Get the work item before acknowledging it to log its details
	var item *WorkItem
	for _, wi := range workQueue.items {
		if wi.ID == request.ID {
			item = wi
			break
		}
	}

	// Log the details and record metrics before acknowledging
	if item != nil {
		log.Printf("Processing ack for item ID: %s, Shape: %v, Position: %v",
			request.ID, item.Ortho.Shape, item.Ortho.Position)

		// Calculate processing duration and record in Prometheus
		if item.DequeuedTime != nil {
			// Convert shape and position to string format for the metric labels
			shapeStr := fmt.Sprintf("%v", item.Ortho.Shape)
			positionStr := fmt.Sprintf("%v", item.Ortho.Position)

			// Record the time from dequeue to ack as processing time
			processingTime := time.Since(*item.DequeuedTime).Seconds()
			orthoProcessingDuration.WithLabelValues(shapeStr, positionStr).Observe(processingTime)

			log.Printf("Recorded processing time for shape=%s, position=%s: %.3f seconds",
				shapeStr, positionStr, processingTime)

			// Increment the processed items by shape×position counter
			itemsProcessedByShapePosition.WithLabelValues(shapeStr, positionStr).Inc()
			log.Printf("Incremented processed items counter for shape=%s, position=%s",
				shapeStr, positionStr)
		}
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

	// Increment the processed items counter when an acknowledgment is successful
	itemsProcessedTotal.Inc()

	c.JSON(http.StatusOK, AckResponse{
		Status:  "success",
		Message: "Work item acknowledged successfully",
	})
}

// handleNack handles the nack endpoint to return work items to the queue
func handleNack(c *gin.Context) {
	var request AckRequest // Using AckRequest since the structure is the same

	// Bind JSON to struct
	if err := c.ShouldBindJSON(&request); err != nil {
		telemetry.LogAndError(c, err, "workserver", "Error parsing nack request")
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request format",
		})
		return
	}

	// Validate ID
	if request.ID == "" {
		log.Printf("Received NACK request with empty ID")
		c.JSON(http.StatusBadRequest, AckResponse{
			Status:  "error",
			Message: "Empty work item ID is not valid",
		})
		return
	}

	// Get the work item before nacking it to log its details
	var item *WorkItem
	for _, wi := range workQueue.items {
		if wi.ID == request.ID {
			item = wi
			break
		}
	}

	// Log the details before nacking
	if item != nil {
		log.Printf("Processing nack for item ID: %s, Shape: %v, Position: %v",
			request.ID, item.Ortho.Shape, item.Ortho.Position)
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

// handleSampleInFlight handles the endpoint to sample a currently processing ortho
func handleSampleInFlight(c *gin.Context) {
	// Get a lock to ensure we have a consistent snapshot
	workQueue.mutex.Lock()
	defer workQueue.mutex.Unlock()

	// Find in-flight items
	var inFlightItems []*WorkItem
	for _, item := range workQueue.items {
		if item.InProgress {
			inFlightItems = append(inFlightItems, item)
		}
	}

	// If no in-flight items, return empty response
	if len(inFlightItems) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "No in-flight items found",
			"count":   0,
			"samples": []interface{}{},
		})
		return
	}

	// Limit the response to a maximum of 5 samples to avoid overwhelming responses
	maxSamples := 5
	if len(inFlightItems) > maxSamples {
		// Take a few samples from the beginning of the list
		inFlightItems = inFlightItems[:maxSamples]
	}

	// Create sample data with details about each in-flight ortho
	samples := make([]gin.H, len(inFlightItems))
	for i, item := range inFlightItems {
		// Create a map of grid values for easier visualization
		gridSample := make([]gin.H, 0, len(item.Ortho.Grid))
		for k, v := range item.Ortho.Grid {
			gridSample = append(gridSample, gin.H{
				"key":   k,
				"value": v,
			})
		}

		// Calculate processing time so far
		var processingTime float64
		if item.DequeuedTime != nil {
			processingTime = time.Since(*item.DequeuedTime).Seconds()
		}

		samples[i] = gin.H{
			"id":                  item.ID,
			"shape":               item.Ortho.Shape,
			"position":            item.Ortho.Position,
			"shell":               item.Ortho.Shell,
			"grid_size":           len(item.Ortho.Grid),
			"grid_sample":         gridSample,
			"processing_time_sec": processingTime,
			"enqueued_at":         item.EnqueuedTime,
			"dequeued_at":         item.DequeuedTime,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Successfully sampled in-flight items",
		"count":   len(samples),
		"samples": samples,
	})
}
