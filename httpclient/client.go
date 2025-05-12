package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
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

// MessageWithAck holds both the deserialized message data and its delivery tag for acknowledgment
type MessageWithAck[T any] struct {
	Data        T
	DeliveryTag uint64
	Channel     *amqp.Channel
}

// Ack acknowledges the message
func (m *MessageWithAck[T]) Ack() error {
	return m.Channel.Ack(m.DeliveryTag, false) // false = don't ack multiple messages
}

// Nack rejects the message and always requeues it
func (m *MessageWithAck[T]) Nack() error {
	return m.Channel.Nack(m.DeliveryTag, false, true) // true = always requeue
}

// RabbitClient provides a simple interface for pushing messages to RabbitMQ
type RabbitClient[T any] struct {
	conn     *amqp.Connection
	channel  *amqp.Channel
	confirms chan amqp.Confirmation
	tracer   trace.Tracer
	url      string     // Store the URL for reconnection purposes
	mu       sync.Mutex // Mutex to protect connection operations
}

// New method to handle reconnection
func (c *RabbitClient[T]) ensureConnected(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if connection is closed
	if c.conn == nil || c.conn.IsClosed() {
		log.Printf("DIAG: Connection is closed or nil, reconnecting to RabbitMQ at %s", c.url)
		conn, err := amqp.Dial(c.url)
		if err != nil {
			log.Printf("DIAG: Failed to reconnect to RabbitMQ: %v", err)
			return fmt.Errorf("failed to reconnect to RabbitMQ: %w", err)
		}
		c.conn = conn

		// Set up connection close notification
		connCloseChan := make(chan *amqp.Error, 1)
		conn.NotifyClose(connCloseChan)
		go func() {
			err := <-connCloseChan
			if err != nil {
				log.Printf("DIAG: RabbitMQ connection closed with error: %v", err)
			} else {
				log.Printf("DIAG: RabbitMQ connection closed gracefully")
			}
		}()

		log.Printf("DIAG: Successfully reconnected to RabbitMQ")
	}

	// Check if channel is closed
	if c.channel == nil || c.channel.IsClosed() {
		log.Printf("DIAG: Channel is closed or nil, creating new channel")
		ch, err := c.conn.Channel()
		if err != nil {
			log.Printf("DIAG: Failed to create new RabbitMQ channel: %v", err)
			return fmt.Errorf("failed to create new channel: %w", err)
		}
		c.channel = ch

		// Set up channel close notification
		chanCloseChan := make(chan *amqp.Error, 1)
		ch.NotifyClose(chanCloseChan)
		go func() {
			err := <-chanCloseChan
			if err != nil {
				log.Printf("DIAG: RabbitMQ channel closed with error: %v", err)
			} else {
				log.Printf("DIAG: RabbitMQ channel closed gracefully")
			}
		}()

		// Enable publisher confirms
		if err := ch.Confirm(false); err != nil {
			log.Printf("DIAG: Failed to enable confirm mode on channel: %v", err)
			ch.Close()
			return fmt.Errorf("failed to put channel in confirm mode: %w", err)
		}
		c.confirms = ch.NotifyPublish(make(chan amqp.Confirmation, 1))
		log.Printf("DIAG: Successfully created new channel with publisher confirms")
	}

	return nil
}

// NewRabbitClient creates a new RabbitMQ client
func NewRabbitClient[T any](url string) (*RabbitClient[T], error) {
	// Initialize tracer
	tracer := otel.Tracer("github.com/crochet/httpclient/rabbit")
	// Create a span for the connection setup
	ctx := context.Background()
	var span trace.Span
	ctx, span = tracer.Start(ctx, "rabbitmq.connect")
	defer span.End()
	// Add detailed connection logging
	log.Printf("DIAG: Attempting to connect to RabbitMQ at URL: %s", url)
	// Add connection details to span
	span.SetAttributes(attribute.String("rabbitmq.url", url))
	conn, err := amqp.Dial(url)
	if err != nil {
		log.Printf("DIAG: RabbitMQ connection error: %v", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to connect to RabbitMQ")
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}
	log.Printf("DIAG: Successfully connected to RabbitMQ at %s", url)
	// Log server properties if available
	if conn.Properties != nil {
		log.Printf("DIAG: Connected to RabbitMQ server with properties: %+v", conn.Properties)
	}
	// Set up connection close notification
	connCloseChan := make(chan *amqp.Error, 1)
	conn.NotifyClose(connCloseChan)
	go func() {
		err := <-connCloseChan
		if err != nil {
			log.Printf("DIAG: RabbitMQ connection closed with error: %v", err)
		} else {
			log.Printf("DIAG: RabbitMQ connection closed gracefully")
		}
	}()
	ch, err := conn.Channel()
	if err != nil {
		log.Printf("DIAG: Failed to open RabbitMQ channel: %v", err)
		conn.Close()
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to open channel")
		return nil, fmt.Errorf("failed to open a channel: %w", err)
	}
	log.Printf("DIAG: Successfully opened RabbitMQ channel")
	// Set up channel close notification
	chanCloseChan := make(chan *amqp.Error, 1)
	ch.NotifyClose(chanCloseChan)
	go func() {
		err := <-chanCloseChan
		if err != nil {
			log.Printf("DIAG: RabbitMQ channel closed with error: %v", err)
		} else {
			log.Printf("DIAG: RabbitMQ channel closed gracefully")
		}
	}()
	// Enable publisher confirms
	if err := ch.Confirm(false); err != nil {
		log.Printf("DIAG: Failed to enable confirm mode on channel: %v", err)
		ch.Close()
		conn.Close()
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to enable confirm mode")
		return nil, fmt.Errorf("failed to put channel in confirm mode: %w", err)
	}
	log.Printf("DIAG: Successfully enabled publisher confirms")
	confirms := ch.NotifyPublish(make(chan amqp.Confirmation, 1))
	span.SetStatus(codes.Ok, "RabbitMQ client initialized successfully")
	return &RabbitClient[T]{
		conn:     conn,
		channel:  ch,
		confirms: confirms,
		tracer:   tracer,
		url:      url, // Store the URL for reconnection
	}, nil
}

// DeclareQueue ensures that the specified queue exists
func (c *RabbitClient[T]) DeclareQueue(ctx context.Context, queueName string) error {
	var span trace.Span
	ctx, span = c.tracer.Start(ctx, "rabbitmq.declare_queue")
	defer span.End()

	// Ensure connection and channel are open
	if err := c.ensureConnected(ctx); err != nil {
		log.Printf("DIAG: Failed to ensure connection before declaring queue %s: %v", queueName, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to ensure connection")
		return err
	}

	log.Printf("DIAG: Declaring queue: %s", queueName)
	log.Printf("DIAG: Connection state before queue declaration - closed: %v", c.conn.IsClosed())
	log.Printf("DIAG: Channel state before queue declaration - closed: %v", c.channel.IsClosed())

	// Important: Use consistent queue settings across all services
	// - durable: true to survive broker restarts
	// - autoDelete: false to not delete when last consumer disconnects
	// - exclusive: false to allow multiple connections to consume
	// - noWait: false to wait for confirmation
	queue, err := c.channel.QueueDeclare(
		queueName, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive - MUST be false to allow multiple services to use it
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		log.Printf("DIAG: Failed to declare queue %s: %v", queueName, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to declare queue")
		return fmt.Errorf("failed to declare queue: %w", err)
	}
	log.Printf("DIAG: Queue %s declared successfully: %+v", queueName, queue)
	span.SetStatus(codes.Ok, "queue declared successfully")
	return nil
}

// PushMessage publishes a message to the specified queue and waits for server acknowledgement
func (c *RabbitClient[T]) PushMessage(ctx context.Context, queueName string, message []byte) error {
	var span trace.Span
	ctx, span = c.tracer.Start(ctx, "rabbitmq.publish_message")
	defer span.End()

	// Ensure connection and channel are open with retries
	maxRetries := 3
	var err error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		err = c.ensureConnected(ctx)
		if err == nil {
			break
		}
		log.Printf("DIAG: Failed to ensure connection before publishing to queue %s (attempt %d/%d): %v",
			queueName, attempt, maxRetries, err)
		time.Sleep(time.Duration(attempt*100) * time.Millisecond) // Backoff
	}

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to ensure connection after multiple attempts")
		return fmt.Errorf("failed to ensure connection after %d attempts: %w", maxRetries, err)
	}

	span.SetAttributes(
		attribute.String("rabbitmq.queue", queueName),
		attribute.Int("rabbitmq.message.size", len(message)),
	)
	requestID := "unknown"
	if ctx.Value("request_id") != nil {
		requestID = fmt.Sprintf("%v", ctx.Value("request_id"))
	}

	log.Printf("DIAG[%s]: Publishing message to queue %s, message size: %d bytes", requestID, queueName, len(message))
	log.Printf("DIAG[%s]: Connection state before publish - closed: %v", requestID, c.conn.IsClosed())
	log.Printf("DIAG[%s]: Channel state before publish - closed: %v", requestID, c.channel.IsClosed())

	// Add content preview for debugging
	if len(message) > 0 {
		previewLength := min(len(message), 500)
		log.Printf("DIAG[%s]: Message content: %s", requestID, string(message[:previewLength]))
	}

	// Declare the queue before publishing
	// Use a separate context with timeout just for queue declaration
	declareCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := c.DeclareQueue(declareCtx, queueName); err != nil {
		log.Printf("DIAG[%s]: Failed to declare queue %s before publishing: %v", requestID, queueName, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to declare queue before publishing")
		return fmt.Errorf("failed to declare queue before publishing: %w", err)
	}

	// Publish the message
	log.Printf("DIAG[%s]: About to publish message to queue %s", requestID, queueName)

	// Create a separate context with timeout just for publishing
	publishCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err = c.channel.PublishWithContext(
		publishCtx,
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
		log.Printf("DIAG[%s]: Failed to publish message: %v", requestID, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to publish message")
		return fmt.Errorf("failed to publish message: %w", err)
	}

	log.Printf("DIAG[%s]: Message published, waiting for confirmation", requestID)

	// Wait for confirmation with a timeout
	select {
	case confirm := <-c.confirms:
		if !confirm.Ack {
			err := fmt.Errorf("message rejected by server")
			log.Printf("DIAG[%s]: Message rejected by server, delivery tag: %d", requestID, confirm.DeliveryTag)
			span.RecordError(err)
			span.SetStatus(codes.Error, "message rejected by server")
			return err
		}
		log.Printf("DIAG[%s]: Message confirmed by server, delivery tag: %d", requestID, confirm.DeliveryTag)
		span.SetStatus(codes.Ok, "message published successfully")
		return nil
	case <-time.After(5 * time.Second):
		err := fmt.Errorf("timeout waiting for confirmation")
		log.Printf("DIAG[%s]: %v", requestID, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "timeout waiting for confirmation")
		return err
	}
}

// PushMessageBatch publishes multiple messages to the specified queue with optional batching
func (c *RabbitClient[T]) PushMessageBatch(ctx context.Context, queueName string, messages [][]byte) error {
	var span trace.Span
	ctx, span = c.tracer.Start(ctx, "rabbitmq.publish_message_batch")
	defer span.End()

	span.SetAttributes(
		attribute.String("rabbitmq.queue", queueName),
		attribute.Int("rabbitmq.batch.size", len(messages)),
	)

	requestID := "unknown"
	if ctx.Value("request_id") != nil {
		requestID = fmt.Sprintf("%v", ctx.Value("request_id"))
	}

	log.Printf("DIAG[%s]: Publishing batch of %d messages to queue %s",
		requestID, len(messages), queueName)

	// First ensure queue exists
	err := c.DeclareQueue(ctx, queueName)
	if err != nil {
		log.Printf("DIAG[%s]: Failed to declare queue %s before batch publish: %v",
			requestID, queueName, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to declare queue")
		return err
	}

	// Check if we need to limit the batch size based on environment variables
	// This is a safety measure in case the higher level batching in services.go
	// doesn't catch all use cases
	maxBatchSizeStr := os.Getenv("FEEDER_BATCH_SIZE")
	maxBatchSize := 1000 // Default batch size
	if batchSize, err := strconv.Atoi(maxBatchSizeStr); err == nil && batchSize > 0 {
		maxBatchSize = batchSize
		log.Printf("DIAG[%s]: Using configured max batch size from env: %d", requestID, maxBatchSize)
	}

	// Check if we need to add a delay between sub-batches
	batchDelayStr := os.Getenv("FEEDER_PUSH_BATCH_DELAY")
	var batchDelay time.Duration
	if delayStr, err := time.ParseDuration(batchDelayStr); err == nil {
		batchDelay = delayStr
		log.Printf("DIAG[%s]: Using configured batch delay from env: %s", requestID, batchDelay)
	} else {
		batchDelay = 200 * time.Millisecond // Default delay
	}

	// Process in sub-batches if needed
	for i := 0; i < len(messages); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(messages) {
			end = len(messages)
		}

		subBatch := messages[i:end]
		log.Printf("DIAG[%s]: Processing sub-batch %d to %d of %d messages",
			requestID, i, end-1, len(messages))

		// Publish each message in the sub-batch individually
		for j, message := range subBatch {
			log.Printf("DIAG[%s]: Publishing message %d/%d to queue %s",
				requestID, i+j+1, len(messages), queueName)

			err := c.PushMessage(ctx, queueName, message)
			if err != nil {
				log.Printf("DIAG[%s]: Failed to publish message %d/%d to queue %s: %v",
					requestID, i+j+1, len(messages), queueName, err)
				span.RecordError(err)
				span.SetStatus(codes.Error, fmt.Sprintf("failed to publish message %d/%d", i+j+1, len(messages)))
				return fmt.Errorf("failed to publish message %d/%d: %w", i+j+1, len(messages), err)
			}
		}

		// Add a delay between sub-batches if this isn't the last sub-batch
		if end < len(messages) && batchDelay > 0 {
			log.Printf("DIAG[%s]: Sleeping for %s before next sub-batch", requestID, batchDelay)
			time.Sleep(batchDelay)
		}
	}

	log.Printf("DIAG[%s]: Successfully published batch of %d messages to queue %s",
		requestID, len(messages), queueName)
	span.SetStatus(codes.Ok, fmt.Sprintf("Successfully published %d messages", len(messages)))
	return nil
}

// PopMessagesFromQueue is a convenience method that retrieves messages from a queue
func (c *RabbitClient[T]) PopMessagesFromQueue(ctx context.Context, queueName string, batchSize int) ([]MessageWithAck[T], error) {
	var span trace.Span
	ctx, span = c.tracer.Start(ctx, "rabbitmq.pop_messages_from_queue")
	defer span.End()

	// Ensure connection and channel are open
	if err := c.ensureConnected(ctx); err != nil {
		log.Printf("DIAG: Failed to ensure connection before popping from queue %s: %v", queueName, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to ensure connection")
		return nil, err
	}

	span.SetAttributes(
		attribute.String("rabbitmq.queue", queueName),
		attribute.Int("rabbitmq.batch.size", batchSize),
	)

	requestID := "unknown"
	if ctx.Value("request_id") != nil {
		requestID = fmt.Sprintf("%v", ctx.Value("request_id"))
	}

	log.Printf("DIAG[%s]: Popping messages from queue %s with batch size %d", requestID, queueName, batchSize)
	log.Printf("DIAG[%s]: Connection state before pop - closed: %v", requestID, c.conn.IsClosed())
	log.Printf("DIAG[%s]: Channel state before pop - closed: %v", requestID, c.channel.IsClosed())

	// Use InspectQueue to get accurate queue statistics
	queueInfo, err := c.InspectQueue(ctx, queueName)
	if err != nil {
		log.Printf("DIAG[%s]: Error inspecting queue %s: %v", requestID, queueName, err)
	} else {
		log.Printf("DIAG[%s]: Queue %s has %d messages and %d consumers before consumption",
			requestID, queueName, queueInfo.Messages, queueInfo.Consumers)
	}

	// Set QoS settings for batch consumption
	log.Printf("DIAG[%s]: Setting QoS with prefetch count %d", requestID, batchSize)
	if err := c.channel.Qos(
		batchSize, // prefetch count
		0,         // prefetch size
		false,     // global
	); err != nil {
		log.Printf("DIAG[%s]: Failed to set QoS: %v", requestID, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to set QoS")
		return nil, fmt.Errorf("failed to set QoS: %w", err)
	}

	log.Printf("DIAG[%s]: QoS set successfully", requestID)

	// Use Get method instead of Consume for non-blocking behavior
	log.Printf("DIAG[%s]: Using Get method to retrieve messages from queue %s", requestID, queueName)

	result := make([]MessageWithAck[T], 0, batchSize)

	// Try to get up to batchSize messages from the queue
	for i := 0; i < batchSize; i++ {
		delivery, ok, err := c.channel.Get(queueName, false) // false = no auto-ack
		if err != nil {
			log.Printf("DIAG[%s]: Error getting message from queue %s: %v", requestID, queueName, err)
			break
		}

		if !ok {
			log.Printf("DIAG[%s]: No more messages available in queue %s after retrieving %d",
				requestID, queueName, len(result))
			break
		}

		// Log message properties
		log.Printf("DIAG[%s]: Retrieved message from queue %s: tag=%d, content-type=%s, length=%d",
			requestID, queueName, delivery.DeliveryTag, delivery.ContentType, len(delivery.Body))

		// Attempt to unmarshal message
		var data T
		if err := json.Unmarshal(delivery.Body, &data); err != nil {
			log.Printf("DIAG[%s]: Failed to unmarshal message body: %v", requestID, err)
			log.Printf("DIAG[%s]: Raw message content: %s", requestID, string(delivery.Body[:min(len(delivery.Body), 500)]))

			// Nack the message to requeue it, or consider moving to a dead-letter queue
			if err := delivery.Nack(false, true); err != nil {
				log.Printf("DIAG[%s]: Failed to nack message: %v", requestID, err)
			}
			continue
		}

		// Add the message to our result set
		result = append(result, MessageWithAck[T]{
			Data:        data,
			DeliveryTag: delivery.DeliveryTag,
			Channel:     c.channel,
		})
	}

	log.Printf("DIAG[%s]: Successfully collected %d messages from queue %s", requestID, len(result), queueName)
	span.SetStatus(codes.Ok, fmt.Sprintf("Successfully popped %d messages", len(result)))
	return result, nil
}

// AckByDeliveryTag acknowledges a message by its delivery tag
func (c *RabbitClient[T]) AckByDeliveryTag(ctx context.Context, tag uint64) error {
	var span trace.Span
	ctx, span = c.tracer.Start(ctx, "rabbitmq.ack_by_delivery_tag")
	defer span.End()

	// Ensure connection and channel are open
	if err := c.ensureConnected(ctx); err != nil {
		log.Printf("DIAG: Failed to ensure connection before acknowledging message: %v", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to ensure connection")
		return err
	}

	span.SetAttributes(
		attribute.Int64("rabbitmq.delivery_tag", int64(tag)),
	)

	requestID := "unknown"
	if ctx.Value("request_id") != nil {
		requestID = fmt.Sprintf("%v", ctx.Value("request_id"))
	}

	log.Printf("DIAG[%s]: Acknowledging message with delivery tag %d", requestID, tag)
	log.Printf("DIAG[%s]: Connection state before ack - closed: %v", requestID, c.conn.IsClosed())
	log.Printf("DIAG[%s]: Channel state before ack - closed: %v", requestID, c.channel.IsClosed())

	err := c.channel.Ack(tag, false) // false means don't acknowledge multiple messages
	if err != nil {
		log.Printf("DIAG[%s]: Failed to acknowledge message: %v", requestID, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to acknowledge message")
		return fmt.Errorf("failed to acknowledge message: %w", err)
	}

	log.Printf("DIAG[%s]: Successfully acknowledged message with delivery tag %d", requestID, tag)
	span.SetStatus(codes.Ok, "Message acknowledged")
	return nil
}

// Close closes the connection and channel
func (c *RabbitClient[T]) Close(ctx context.Context) error {
	ctx, span := c.tracer.Start(ctx, "rabbitmq.close")
	defer span.End()

	log.Printf("DIAG: Closing RabbitMQ connection and channel")

	if c.channel != nil {
		log.Printf("DIAG: Closing channel (channel closed: %v)", c.channel.IsClosed())
		if !c.channel.IsClosed() {
			if err := c.channel.Close(); err != nil {
				log.Printf("DIAG: Error closing channel: %v", err)
				span.RecordError(err)
				span.SetStatus(codes.Error, "failed to close channel")
				return fmt.Errorf("failed to close channel: %w", err)
			}
			log.Printf("DIAG: Channel closed successfully")
		} else {
			log.Printf("DIAG: Channel was already closed")
		}
	} else {
		log.Printf("DIAG: No channel to close")
	}

	if c.conn != nil {
		log.Printf("DIAG: Closing connection (connection closed: %v)", c.conn.IsClosed())
		if !c.conn.IsClosed() {
			if err := c.conn.Close(); err != nil {
				log.Printf("DIAG: Error closing connection: %v", err)
				span.RecordError(err)
				span.SetStatus(codes.Error, "failed to close connection")
				return fmt.Errorf("failed to close connection: %w", err)
			}
			log.Printf("DIAG: Connection closed successfully")
		} else {
			log.Printf("DIAG: Connection was already closed")
		}
	} else {
		log.Printf("DIAG: No connection to close")
	}

	span.SetStatus(codes.Ok, "RabbitMQ connection closed successfully")
	return nil
}

// QueueInfo contains detailed information about a RabbitMQ queue
type QueueInfo struct {
	Messages               int
	Consumers              int
	MessagesReady          int
	MessagesUnacknowledged int
}

// InspectQueue gets detailed information about a queue
func (c *RabbitClient[T]) InspectQueue(ctx context.Context, queueName string) (QueueInfo, error) {
	var span trace.Span
	ctx, span = c.tracer.Start(ctx, "rabbitmq.inspect_queue")
	defer span.End()

	requestID := "unknown"
	if ctx.Value("request_id") != nil {
		requestID = fmt.Sprintf("%v", ctx.Value("request_id"))
	}

	// Ensure connection and channel are open
	if err := c.ensureConnected(ctx); err != nil {
		log.Printf("DIAG[%s]: Failed to ensure connection before inspecting queue %s: %v",
			requestID, queueName, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to ensure connection")
		return QueueInfo{}, err
	}

	log.Printf("DIAG[%s]: Inspecting queue %s", requestID, queueName)

	// Use QueueInspect to get queue details
	queue, err := c.channel.QueueInspect(queueName)
	if err != nil {
		log.Printf("DIAG[%s]: Failed to inspect queue %s: %v", requestID, queueName, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to inspect queue")
		return QueueInfo{}, fmt.Errorf("failed to inspect queue: %w", err)
	}

	info := QueueInfo{
		Messages:               queue.Messages,
		Consumers:              queue.Consumers,
		MessagesReady:          queue.Messages, // In the basic case, these are equal
		MessagesUnacknowledged: 0,              // We can't get this directly from QueueInspect
	}

	log.Printf("DIAG[%s]: Queue %s has %d messages and %d consumers",
		requestID, queueName, queue.Messages, queue.Consumers)

	span.SetStatus(codes.Ok, "successfully inspected queue")
	return info, nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
