package rabbitmq

import (
	"context"
	"crochet/pkg/config"
	"fmt"
	"log"

	"github.com/rabbitmq/amqp091-go"
)

type Queue interface {
	Publish(ctx context.Context, queueName string, body []byte) error
	ConsumeOne(ctx context.Context, queueName string) ([]byte, error)
	GetQueueDepth(queueName string) (int, error)
}

type queue struct {
	conn *amqp091.Connection
}

func NewQueue(conn *amqp091.Connection) Queue {
	return &queue{conn: conn}
}

func (q *queue) Publish(ctx context.Context, queueName string, body []byte) error {
	ch, err := q.conn.Channel()
	if err != nil {
		log.Printf("[RabbitMQ] Failed to open channel for publish: %v", err)
		return err
	}
	defer ch.Close()
	_, err = ch.QueueDeclare(queueName, false, false, false, false, nil)
	if err != nil {
		log.Printf("[RabbitMQ] Failed to declare queue '%s': %v", queueName, err)
		return err
	}
	err = ch.PublishWithContext(ctx, "", queueName, false, false,
		amqp091.Publishing{ContentType: "text/plain", Body: body},
	)
	if err != nil {
		log.Printf("[RabbitMQ] Failed to publish to queue '%s': %v", queueName, err)
	}
	return err
}

func (q *queue) ConsumeOne(ctx context.Context, queueName string) ([]byte, error) {
	ch, err := q.conn.Channel()
	if err != nil {
		log.Printf("[RabbitMQ] Failed to open channel for consume: %v", err)
		return nil, err
	}
	defer ch.Close()
	msgs, err := ch.Consume(queueName, "", false, false, false, false, nil)
	if err != nil {
		log.Printf("[RabbitMQ] Failed to consume from queue '%s': %v", queueName, err)
		return nil, err
	}
	select {
	case msg := <-msgs:
		err := msg.Ack(false)
		if err != nil {
			log.Printf("[RabbitMQ] Failed to ack message from queue '%s': %v", queueName, err)
		}
		return msg.Body, nil
	case <-ctx.Done():
		log.Printf("[RabbitMQ] Context cancelled while consuming from queue '%s'", queueName)
		return nil, ctx.Err()
	}
}

func (q *queue) GetQueueDepth(queueName string) (int, error) {
	ch, err := q.conn.Channel()
	if err != nil {
		log.Printf("[RabbitMQ] Failed to open channel for queue depth: %v", err)
		return 0, err
	}
	defer ch.Close()
	queue, err := ch.QueueDeclarePassive(queueName, false, false, false, false, nil)
	if err != nil {
		log.Printf("[RabbitMQ] Failed to declare passive queue '%s': %v", queueName, err)
		return 0, err
	}
	return queue.Messages, nil
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
