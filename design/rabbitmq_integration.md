# RabbitMQ Integration Plan for Crochet

## 1. Docker Compose Integration
- Add a `rabbitmq` service to `docker-compose.yml` using the `rabbitmq:3-management` image.
- Expose ports 5672 (AMQP) and 15672 (management UI).
- Set default user and password via environment variables.
- Add to `app-network`.

## 2. Configuration (`config.go` and `.env`)
- Add RabbitMQ connection parameters to `.env`:
  - RABBITMQ_HOST
  - RABBITMQ_PORT
  - RABBITMQ_USER
  - RABBITMQ_PASS
  - RABBITMQ_VHOST
- Update `Config` struct in `config.go` to include RabbitMQ config.
- Update `Load()` to read and validate these new variables.

## 3. Dependency Injection (`main.go`)
- Add a provider for a RabbitMQ connection (e.g., using `github.com/rabbitmq/amqp091-go`).
- Provide this connection via fx to handlers.
- Ensure proper lifecycle management (connect on start, close on stop).

## 4. Handler Example (`handler.go`)
- Inject RabbitMQ connection/channel into handler.
- In `/ping`, demonstrate:
  - Publishing a message to a queue.
  - Consuming a message from the queue.
  - Acknowledging the message.
- Ensure this is testable and does not break existing tests.

## 5. Testing
- **Unit:** Mock the RabbitMQ connection and test handler logic.
- **Integration:** Use RabbitMQ test container (add to test setup).
- **E2E:** Ensure RabbitMQ is up in the compose stack and `/ping` exercises the queue.

## 6. Telemetry
- Add a Prometheus gauge for RabbitMQ queue depth.
- Expose this metric in `/metrics` for Grafana.
- Add a panel to the Grafana dashboard to visualize queue depth.

---

This plan covers all required files and layers, and includes telemetry for queue depth monitoring in Grafana.
