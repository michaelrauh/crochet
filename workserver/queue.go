package main

import (
	"crochet/types"
	"fmt"
	"log"
	"sync"
	"time"
)

// WorkQueue provides a thread-safe queue for work items
type WorkQueue struct {
	items            []*WorkItem
	mutex            sync.Mutex
	requeueTimeoutMS int64 // Timeout in milliseconds
}

// NewWorkQueue creates a new work queue
func NewWorkQueue(requeueTimeoutSeconds int) *WorkQueue {
	return &WorkQueue{
		items:            make([]*WorkItem, 0),
		requeueTimeoutMS: int64(requeueTimeoutSeconds * 1000),
	}
}

// Push adds multiple orthos to the work queue
func (q *WorkQueue) Push(orthos []types.Ortho) []string {
	if len(orthos) == 0 {
		return []string{}
	}
	q.mutex.Lock()
	defer q.mutex.Unlock()
	now := time.Now()
	ids := make([]string, len(orthos))
	for i, ortho := range orthos {
		// Generate a unique ID if the ortho doesn't have one
		itemID := ortho.ID
		if itemID == "" {
			// Create a timestamp-based unique ID
			itemID = fmt.Sprintf("work-%d-%d", now.UnixNano(), i)

			// Make a copy of the ortho to avoid modifying the original
			orthoWithID := ortho
			orthoWithID.ID = itemID
			ortho = orthoWithID
		}

		workItem := &WorkItem{
			Ortho:        ortho,
			EnqueuedTime: now,
			DequeuedTime: nil,
			ID:           itemID,
			InProgress:   false,
		}
		q.items = append(q.items, workItem)
		ids[i] = itemID
	}
	log.Printf("Added %d items to work queue. Total items: %d", len(orthos), len(q.items))
	return ids
}

// Pop removes and returns the first available ortho from the queue
func (q *WorkQueue) Pop() (*WorkItem, bool) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	// First, check for timed-out items and requeue them
	now := time.Now().UnixMilli()
	for _, item := range q.items {
		if item.InProgress {
			if item.DequeuedTime != nil {
				timeoutThreshold := item.DequeuedTime.UnixMilli() + q.requeueTimeoutMS
				if now > timeoutThreshold {
					log.Printf("Requeuing timed-out work item with ID: %s", item.ID)
					item.InProgress = false
					item.DequeuedTime = nil
				}
			}
		}
	}

	// Then try to find an available item
	for _, item := range q.items {
		if !item.InProgress {
			dequeueTime := time.Now()
			item.DequeuedTime = &dequeueTime
			item.InProgress = true
			return item, true
		}
	}

	// No available items
	return nil, false
}

// Ack acknowledges that a work item has been processed
func (q *WorkQueue) Ack(id string) bool {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	for i, item := range q.items {
		if item.ID == id && item.InProgress {
			// Log the shape and position of the ortho being acknowledged
			log.Printf("Acknowledging work item ID: %s, Shape: %v, Position: %v",
				id, item.Ortho.Shape, item.Ortho.Position)

			// Remove the item from the queue by swapping it with the last element
			// and truncating the slice - more efficient than creating a new slice
			lastIndex := len(q.items) - 1
			q.items[i] = q.items[lastIndex]
			q.items = q.items[:lastIndex]
			return true
		}
	}

	log.Printf("Failed to acknowledge work item with ID: %s, item not found or not in progress", id)
	return false
}

// Nack marks a work item as no longer in progress, making it available for processing again
func (q *WorkQueue) Nack(id string) bool {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	for _, item := range q.items {
		if item.ID == id && item.InProgress {
			// Log the shape and position of the ortho being negative acknowledged
			log.Printf("Negative acknowledging work item ID: %s, Shape: %v, Position: %v",
				id, item.Ortho.Shape, item.Ortho.Position)

			item.InProgress = false
			item.DequeuedTime = nil
			return true
		}
	}

	log.Printf("Failed to negative acknowledge work item with ID: %s, item not found or not in progress", id)
	return false
}

// Count returns the total number of items in the queue
func (q *WorkQueue) Count() int {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	return len(q.items)
}

// CountQueued returns the count of items in the queue that are not in progress
func (q *WorkQueue) CountQueued() int {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	count := 0
	for _, item := range q.items {
		if !item.InProgress {
			count++
		}
	}
	return count
}

// CountInFlight returns the count of items that are currently being processed
func (q *WorkQueue) CountInFlight() int {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	count := 0
	for _, item := range q.items {
		if item.InProgress {
			count++
		}
	}
	return count
}

// CountByShape returns a map of shape to count of items with that shape
func (q *WorkQueue) CountByShape() map[string]int {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	// Map to store counts by shape
	shapeCounts := make(map[string]int)
	for _, item := range q.items {
		// Simply use the string representation of the shape array as the key
		shape := fmt.Sprintf("%v", item.Ortho.Shape)
		shapeCounts[shape]++
	}
	return shapeCounts
}

// CountQueuedByShape returns a map of shape to count of queued items (not in progress)
func (q *WorkQueue) CountQueuedByShape() map[string]int {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	// Map to store counts by shape
	shapeCounts := make(map[string]int)
	for _, item := range q.items {
		if !item.InProgress {
			// Only count items that are not in progress
			shape := fmt.Sprintf("%v", item.Ortho.Shape)
			shapeCounts[shape]++
		}
	}
	return shapeCounts
}

// CountInFlightByShape returns a map of shape to count of in-flight items
func (q *WorkQueue) CountInFlightByShape() map[string]int {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	// Map to store counts by shape
	shapeCounts := make(map[string]int)
	for _, item := range q.items {
		if item.InProgress {
			// Only count items that are in progress
			shape := fmt.Sprintf("%v", item.Ortho.Shape)
			shapeCounts[shape]++
		}
	}
	return shapeCounts
}

// CountByShapeName returns the count of items with the specified shape name
func (q *WorkQueue) CountByShapeName(shapeName string) int {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	count := 0
	for _, item := range q.items {
		// Use the consistent string representation of the shape array as the key
		shape := fmt.Sprintf("%v", item.Ortho.Shape)
		if shape == shapeName {
			count++
		}
	}

	return count
}

// CountQueuedByShapeAndPosition returns a map of shape+position to count of queued items
func (q *WorkQueue) CountQueuedByShapeAndPosition() map[[2]string]int {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	locationCounts := make(map[[2]string]int)

	for _, item := range q.items {
		if !item.InProgress {
			// Only count items that are not in progress
			shapeKey := fmt.Sprintf("%v", item.Ortho.Shape)
			posKey := fmt.Sprintf("%v", item.Ortho.Position)

			// Create a composite key for shape+position
			locationKey := [2]string{shapeKey, posKey}
			locationCounts[locationKey]++
		}
	}

	return locationCounts
}

// CountInFlightByShapeAndPosition returns a map of shape+position to count of in-flight items
func (q *WorkQueue) CountInFlightByShapeAndPosition() map[[2]string]int {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	locationCounts := make(map[[2]string]int)
	for _, item := range q.items {
		if item.InProgress {
			// Only count items that are currently being processed
			shapeKey := fmt.Sprintf("%v", item.Ortho.Shape)
			posKey := fmt.Sprintf("%v", item.Ortho.Position)
			// Create a composite key for shape+position
			locationKey := [2]string{shapeKey, posKey}
			locationCounts[locationKey]++
		}
	}
	return locationCounts
}

// CountProcessedByShapeAndPosition is a placeholder to return shape×position metrics
// Since we're only incrementing counters at ACK time in the handler directly,
// we return an empty map here as the real counting is done elsewhere
func (q *WorkQueue) CountProcessedByShapeAndPosition() map[[2]string]int {
	// This is a placeholder function
	// The actual counting is done in the handleAck function
	// where we increment Prometheus counters directly
	return make(map[[2]string]int)
}
