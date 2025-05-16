package rabbitmq

import (
	"context"
	"crochet/pkg/config"
	"fmt"
	"log"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

type ChannelInterface interface {
	Close() error
	Qos(prefetchCount, prefetchSize int, global bool) error
	Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp091.Table) (<-chan amqp091.Delivery, error)
}

type Queue interface {
	CreateChannel() (ChannelInterface, error)
	Publish(ctx context.Context, ch ChannelInterface, queueName string, body []byte) error
	ConsumeOne(ctx context.Context, ch ChannelInterface, queueName string) (*amqp091.Delivery, error)
	ConsumeBatch(ctx context.Context, ch ChannelInterface, queueName string, max int) ([]amqp091.Delivery, error)
}

type queue struct {
	conn *amqp091.Connection
}

func NewQueue(conn *amqp091.Connection) Queue {
	return &queue{conn: conn}
}

func (q *queue) CreateChannel() (ChannelInterface, error) {
	ch, err := q.conn.Channel()
	if err != nil {
		log.Printf("[RabbitMQ] Failed to open channel: %v", err)
	}
	return ch, err
}

func (q *queue) Publish(ctx context.Context, ch ChannelInterface, queueName string, body []byte) error {
	amqpCh, ok := ch.(*amqp091.Channel)
	if !ok {
		return fmt.Errorf("invalid channel type")
	}
	_, err := amqpCh.QueueDeclare(queueName, false, false, false, false, nil)
	if err != nil {
		log.Printf("[RabbitMQ] Failed to declare queue '%s': %v", queueName, err)
		return err
	}

	err = amqpCh.PublishWithContext(ctx, "", queueName, false, false,
		amqp091.Publishing{ContentType: "text/plain", Body: body},
	)
	if err != nil {
		log.Printf("[RabbitMQ] Failed to publish to queue '%s': %v", queueName, err)
	}
	return err
}

func (q *queue) ConsumeOne(ctx context.Context, ch ChannelInterface, queueName string) (*amqp091.Delivery, error) {
	msgs, err := ch.Consume(queueName, "", false, false, false, false, nil)
	if err != nil {
		log.Printf("[RabbitMQ] Failed to consume from queue '%s': %v", queueName, err)
		return nil, err
	}

	select {
	case msg := <-msgs:
		return &msg, nil
	case <-ctx.Done():
		log.Printf("[RabbitMQ] Context cancelled while consuming from queue '%s'", queueName)
		return nil, ctx.Err()
	}
}

func (q *queue) ConsumeBatch(ctx context.Context, ch ChannelInterface, queueName string, max int) ([]amqp091.Delivery, error) {
	return q.consumeBatchWithQos(ctx, ch, queueName, max, func(ch ChannelInterface, max int) error {
		return ch.Qos(max, 0, false)
	})
}

func (q *queue) consumeBatchWithQos(
	ctx context.Context,
	ch ChannelInterface,
	queueName string,
	max int,
	setQos func(ch ChannelInterface, max int) error,
) ([]amqp091.Delivery, error) {
	if err := setQos(ch, max); err != nil {
		return nil, fmt.Errorf("failed to set prefetch: %w", err)
	}
	msgs, err := ch.Consume(queueName, "", false, false, false, false, nil)
	if err != nil {
		log.Printf("[RabbitMQ] Failed to consume from queue '%s': %v", queueName, err)
		return nil, err
	}
	batch := make([]amqp091.Delivery, 0, max)
	for i := 0; i < max; i++ {
		select {
		case msg, ok := <-msgs:
			if !ok {
				return batch, nil
			}
			batch = append(batch, msg)
			if len(batch) == max {
				return batch, nil
			}
		case <-ctx.Done():
			return batch, ctx.Err()
		case <-time.After(200 * time.Millisecond):
			return batch, nil
		}
	}
	return batch, nil
}

func NewConnection(cfg *config.Config) (*amqp091.Connection, error) {
	url := fmt.Sprintf(
		"amqp://%s:%s@%s:%d%s",
		cfg.RabbitMQ.User,
		cfg.RabbitMQ.Pass,
		cfg.RabbitMQ.Host,
		cfg.RabbitMQ.Port,
		cfg.RabbitMQ.VHost,
	)
	return amqp091.Dial(url)
}

func CloseConnection(conn *amqp091.Connection) error {
	return conn.Close()
}
