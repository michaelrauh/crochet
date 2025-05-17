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
	log.Printf("[RabbitMQ] Creating new channel from connection %p", q.conn)
	ch, err := q.conn.Channel()
	if err != nil {
		log.Printf("[RabbitMQ] Failed to open channel: %v", err)
	} else {
		log.Printf("[RabbitMQ] Successfully created channel %p", ch)
	}
	return ch, err
}

func (q *queue) Publish(ctx context.Context, ch ChannelInterface, queueName string, body []byte) error {
	log.Printf("[RabbitMQ] Starting to publish message to queue '%s': %s", queueName, string(body))

	amqpCh, ok := ch.(*amqp091.Channel)
	if !ok {
		log.Printf("[RabbitMQ] ERROR: Invalid channel type for queue '%s'", queueName)
		return fmt.Errorf("invalid channel type")
	}

	log.Printf("[RabbitMQ] Declaring queue '%s'", queueName)
	_, err := amqpCh.QueueDeclare(queueName, false, false, false, false, nil)
	if err != nil {
		log.Printf("[RabbitMQ] Failed to declare queue '%s': %v", queueName, err)
		return err
	}

	log.Printf("[RabbitMQ] Publishing message to queue '%s'", queueName)
	err = amqpCh.PublishWithContext(ctx, "", queueName, false, false,
		amqp091.Publishing{ContentType: "text/plain", Body: body},
	)

	if err != nil {
		log.Printf("[RabbitMQ] Failed to publish to queue '%s': %v", queueName, err)
	} else {
		log.Printf("[RabbitMQ] Successfully published message to queue '%s'", queueName)
	}
	return err
}

func (q *queue) ConsumeOne(ctx context.Context, ch ChannelInterface, queueName string) (*amqp091.Delivery, error) {
	log.Printf("[RabbitMQ] Attempting to consume one message from queue '%s'", queueName)

	// Check queue status if possible
	amqpCh, ok := ch.(*amqp091.Channel)
	if ok {
		queue, err := amqpCh.QueueInspect(queueName)
		if err != nil {
			log.Printf("[RabbitMQ] Error inspecting queue '%s': %v", queueName, err)
		} else {
			log.Printf("[RabbitMQ] Queue '%s' status before consume: Messages: %d, Consumers: %d",
				queueName, queue.Messages, queue.Consumers)
		}
	}

	msgs, err := ch.Consume(queueName, "", false, false, false, false, nil)
	if err != nil {
		log.Printf("[RabbitMQ] Failed to consume from queue '%s': %v", queueName, err)
		return nil, err
	}

	log.Printf("[RabbitMQ] Waiting for message from queue '%s'", queueName)
	select {
	case msg := <-msgs:
		log.Printf("[RabbitMQ] Successfully received message from queue '%s': %s",
			queueName, string(msg.Body))
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
