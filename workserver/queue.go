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
			// Remove the item from the queue by swapping it with the last element
			// and truncating the slice - more efficient than creating a new slice
			lastIndex := len(q.items) - 1
			q.items[i] = q.items[lastIndex]
			q.items = q.items[:lastIndex]
			log.Printf("Acknowledged and removed work item with ID: %s", id)
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
			item.InProgress = false
			item.DequeuedTime = nil
			log.Printf("Negative acknowledged work item with ID: %s", id)
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
