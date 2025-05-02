package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type ClientOptions struct {
	DialTimeout   time.Duration
	DialKeepAlive time.Duration
	MaxIdleConns  int
	ClientTimeout time.Duration
}

func DefaultClientOptions() ClientOptions {
	return ClientOptions{
		DialTimeout:   10 * time.Second,
		DialKeepAlive: 30 * time.Second,
		MaxIdleConns:  10,
		ClientTimeout: 30 * time.Second,
	}
}

type GenericClient[T any] struct {
	httpClient *http.Client
}

func NewGenericClient(options ClientOptions) *GenericClient[any] {
	client := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
		Timeout:   options.ClientTimeout,
	}

	return &GenericClient[any]{
		httpClient: client,
	}
}

func NewDefaultGenericClient[T any]() *GenericClient[T] {
	return &GenericClient[T]{
		httpClient: &http.Client{
			Transport: otelhttp.NewTransport(http.DefaultTransport),
			Timeout:   DefaultClientOptions().ClientTimeout,
		},
	}
}

func (c *GenericClient[T]) GenericCall(ctx context.Context, method, url string, payload []byte) (T, error) {
	var result T

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(payload))
	if err != nil {
		return result, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return result, fmt.Errorf("error calling service: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	log.Printf("Response from service %s: %s", url, string(body))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return result, fmt.Errorf("service error: %s, Status Code: %d", string(body), resp.StatusCode)
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return result, fmt.Errorf("invalid response: %w", err)
	}

	return result, nil
}

// RabbitClient provides a simple interface for pushing messages to RabbitMQ
type RabbitClient struct {
	conn     *amqp.Connection
	channel  *amqp.Channel
	confirms chan amqp.Confirmation
	tracer   trace.Tracer
}

// NewRabbitClient creates a new RabbitMQ client
func NewRabbitClient(url string) (*RabbitClient, error) {
	// Initialize tracer
	tracer := otel.Tracer("github.com/crochet/httpclient/rabbit")

	// Create a span for the connection setup
	ctx := context.Background()
	ctx, span := tracer.Start(ctx, "rabbitmq.connect")
	defer span.End()

	// Add connection details to span
	span.SetAttributes(attribute.String("rabbitmq.url", url))

	conn, err := amqp.Dial(url)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to connect to RabbitMQ")
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to open channel")
		return nil, fmt.Errorf("failed to open a channel: %w", err)
	}

	// Enable publisher confirms
	if err := ch.Confirm(false); err != nil {
		ch.Close()
		conn.Close()
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to enable confirm mode")
		return nil, fmt.Errorf("failed to put channel in confirm mode: %w", err)
	}

	confirms := ch.NotifyPublish(make(chan amqp.Confirmation, 1))
	span.SetStatus(codes.Ok, "RabbitMQ client initialized successfully")

	return &RabbitClient{
		conn:     conn,
		channel:  ch,
		confirms: confirms,
		tracer:   tracer,
	}, nil
}

// DeclareQueue declares a queue to ensure it exists
func (c *RabbitClient) DeclareQueue(queueName string) error {
	ctx := context.Background()
	ctx, span := c.tracer.Start(ctx, "rabbitmq.declare_queue")
	defer span.End()

	span.SetAttributes(attribute.String("rabbitmq.queue", queueName))

	_, err := c.channel.QueueDeclare(
		queueName, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to declare queue")
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	span.SetStatus(codes.Ok, "Queue declared successfully")
	return nil
}

// PushMessage publishes a message to the specified queue and waits for server acknowledgement
func (c *RabbitClient) PushMessage(ctx context.Context, queueName string, message []byte) error {
	ctx, span := c.tracer.Start(ctx, "rabbitmq.publish_message")
	defer span.End()

	span.SetAttributes(
		attribute.String("rabbitmq.queue", queueName),
		attribute.Int("rabbitmq.message.size", len(message)),
	)

	// Publish the message
	err := c.channel.PublishWithContext(
		ctx,
		"",        // exchange
		queueName, // routing key
		true,      // mandatory - return message if it can't be delivered to a queue
		false,     // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         message,
			DeliveryMode: amqp.Persistent, // Make message persistent
		},
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to publish message")
		return fmt.Errorf("failed to publish message: %w", err)
	}

	// Wait for confirmation with a timeout
	select {
	case confirm := <-c.confirms:
		if !confirm.Ack {
			err := fmt.Errorf("message rejected by server")
			span.RecordError(err)
			span.SetStatus(codes.Error, "message rejected by server")
			return err
		}
		span.SetStatus(codes.Ok, "Message published and acknowledged")
		return nil
	case <-ctx.Done():
		err := fmt.Errorf("confirmation timed out: %w", ctx.Err())
		span.RecordError(err)
		span.SetStatus(codes.Error, "confirmation timed out")
		return err
	case <-time.After(5 * time.Second):
		err := fmt.Errorf("confirmation timed out after 5 seconds")
		span.RecordError(err)
		span.SetStatus(codes.Error, "confirmation timed out after 5 seconds")
		return err
	}
}

// PushMessageBatch publishes multiple messages to the specified queue and waits for all acknowledgements
func (c *RabbitClient) PushMessageBatch(ctx context.Context, queueName string, messages [][]byte) error {
	ctx, span := c.tracer.Start(ctx, "rabbitmq.publish_batch")
	defer span.End()

	span.SetAttributes(
		attribute.String("rabbitmq.queue", queueName),
		attribute.Int("rabbitmq.batch.size", len(messages)),
	)

	// Set up a confirmation counter
	publishCount := 0
	confirmCount := 0

	// Create a channel to collect confirmation results
	confirmResults := make(chan error, len(messages))

	// Set up a goroutine to collect confirmations
	go func() {
		for publishCount > confirmCount {
			select {
			case confirm := <-c.confirms:
				confirmCount++
				if !confirm.Ack {
					confirmResults <- fmt.Errorf("message %d was rejected by the server", confirm.DeliveryTag)
				}
			case <-ctx.Done():
				confirmResults <- fmt.Errorf("context canceled while waiting for confirmations: %w", ctx.Err())
				return
			case <-time.After(5 * time.Second):
				confirmResults <- fmt.Errorf("timed out waiting for message confirmations")
				return
			}
		}
		close(confirmResults)
	}()

	// Publish all messages first
	for i, message := range messages {
		// Create child span for each message
		publishCtx, msgSpan := c.tracer.Start(ctx, "rabbitmq.publish_batch_message")
		msgSpan.SetAttributes(
			attribute.Int("rabbitmq.message.index", i),
			attribute.Int("rabbitmq.message.size", len(message)),
		)

		err := c.channel.PublishWithContext(
			publishCtx,
			"",        // exchange
			queueName, // routing key
			true,      // mandatory
			false,     // immediate
			amqp.Publishing{
				ContentType:  "application/json",
				Body:         message,
				DeliveryMode: amqp.Persistent, // Make message persistent
			},
		)
		if err != nil {
			msgSpan.RecordError(err)
			msgSpan.SetStatus(codes.Error, "failed to publish message")
			msgSpan.End()
			span.RecordError(err)
			span.SetStatus(codes.Error, "batch publish failed")
			return fmt.Errorf("failed to publish message %d: %w", i, err)
		}
		msgSpan.SetStatus(codes.Ok, "Message published")
		msgSpan.End()
		publishCount++
	}

	// Wait for and collect all confirmation results
	for err := range confirmResults {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "batch publish failed during confirmation")
			return err
		}
	}

	span.SetStatus(codes.Ok, "Batch published and acknowledged")
	return nil
}

// Close closes the connection and channel
func (c *RabbitClient) Close() error {
	ctx := context.Background()
	ctx, span := c.tracer.Start(ctx, "rabbitmq.close")
	defer span.End()

	if c.channel != nil {
		if err := c.channel.Close(); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to close channel")
			return fmt.Errorf("failed to close channel: %w", err)
		}
	}

	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to close connection")
			return fmt.Errorf("failed to close connection: %w", err)
		}
	}

	span.SetStatus(codes.Ok, "RabbitMQ connection closed successfully")
	return nil
}
