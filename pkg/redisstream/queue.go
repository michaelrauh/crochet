package redisstream

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type Queue struct {
	client *redis.Client
}

func NewQueue(client *redis.Client) *Queue {
	return &Queue{client: client}
}

func (q *Queue) PublishBatch(ctx context.Context, streamName string, valuesList []map[string]interface{}) error {
	for _, values := range valuesList {
		args := &redis.XAddArgs{
			Stream: streamName,
			Values: values,
		}
		err := q.client.XAdd(ctx, args).Err()
		if err != nil {
			return err
		}
	}
	return nil
}

func (q *Queue) ConsumeBatch(ctx context.Context, streamName, group, consumer string, count int) ([]redis.XMessage, error) {
	// Use the client directly
	res, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{streamName, ">"},
		Count:    int64(count),
		Block:    0,
	}).Result()

	if err != nil || len(res) == 0 {
		return nil, err
	}
	return res[0].Messages, nil
}

func (q *Queue) Ack(ctx context.Context, streamName, group string, ids ...string) error {
	return q.client.XAck(ctx, streamName, group, ids...).Err()
}

func (q *Queue) Close() error {
	return q.client.Close()
}

func NewClient(opt *redis.Options) *redis.Client {
	return redis.NewClient(opt)
}
