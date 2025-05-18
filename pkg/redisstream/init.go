package redisstream

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// InitializeStream sets up the Redis stream and consumer group if they don't exist
func InitializeStream(client *redis.Client, streamName, groupName string) error {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if the stream exists, if not, create it
	info := client.XInfoStream(ctx, streamName)
	if info.Err() != nil && info.Err().Error() == "ERR no such key" {
		// Stream doesn't exist, create it with an initial message
		log.Printf("Creating Redis stream: %s", streamName)
		_, err := client.XAdd(ctx, &redis.XAddArgs{
			Stream: streamName,
			ID:     "*",
			Values: map[string]interface{}{"init": "true"},
		}).Result()
		if err != nil {
			log.Printf("Failed to create Redis stream: %v", err)
			return err
		}
	} else if info.Err() != nil {
		log.Printf("Error checking Redis stream: %v", info.Err())
		return info.Err()
	}

	// Create the consumer group if it doesn't exist
	_, err := client.XGroupCreate(ctx, streamName, groupName, "0").Result()
	if err != nil {
		// If the error is not "BUSYGROUP Consumer Group name already exists", return it
		if err.Error() != "BUSYGROUP Consumer Group name already exists" {
			log.Printf("Error creating consumer group: %v", err)
			return err
		}
		log.Printf("Consumer group %s already exists for stream %s", groupName, streamName)
	} else {
		log.Printf("Created consumer group %s for stream %s", groupName, streamName)
	}

	return nil
}
