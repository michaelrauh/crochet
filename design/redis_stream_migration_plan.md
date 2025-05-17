# Migration Plan: RabbitMQ to Redis Streams

## Stage 1: Add Redis Streams Service
- Add Redis (with AOF enabled for flush-to-disk) to docker-compose.yml.
- Add Go Redis client dependency (e.g., github.com/redis/go-redis/v9).
- Add Redis config to config package.

**Validation:**
- Run `docker-compose up`.
- Connect to Redis CLI: `docker exec -it <redis-container> redis-cli`.
- Run `CONFIG GET appendonly` and ensure it returns `appendonly = yes`.

---

## Stage 2: Implement Redis Stream Queue Interface
- Create `pkg/redisstream/queue.go` implementing a `Queue` interface similar to RabbitMQ.
- Implement `Publish` and `publishBatch` using `XADD ... MKSTREAM` and `WAIT` for flush-to-disk.
- Implement `ConsumeOne` and `ConsumeBatch` using `XREADGROUP` with manual ack (`XACK`).
- All producers/consumers use the same group (e.g., `db`).
- On produce, return the Redis Stream entry ID for later ack.

**Validation:**
- Write a unit test that:
  - Publishes a message and checks it appears in the stream.
  - Simulates a consumer that reads and acks the message.
  - Verifies the message is not delivered again after ack.

---

## Stage 3: Switch Envelope Publishing to Redis Streams
- Update envelope publishing code to use the new Redis Stream queue interface.
- Ensure all messages are published to the `db` stream.
- Ensure the returned entry ID is passed back for later ack.

**Validation:**
- Run existing unit and integration tests for envelope publishing.
- Add a test that fails if Redis is not configured for AOF or if `WAIT` does not confirm flush.

---

## Stage 4: Update Consumers to Use Redis Streams
- Update all consumers to use `XREADGROUP` with manual ack (`XACK`).
- Ensure all consumers are in the same group (`db`).
- Ensure consumers can handle re-delivery if not acked.

**Validation:**
- Run integration tests to verify:
  - Messages are only delivered once after ack.
  - Unacked messages are re-delivered.
  - All consumers see the same group state.

---

## Stage 5: Remove RabbitMQ
- Remove RabbitMQ service from docker-compose.yml.
- Remove RabbitMQ code and dependencies.
- Update documentation.

**Validation:**
- Run `docker-compose up` and verify the system works end-to-end with only Redis Streams.
- Run all tests and ensure no RabbitMQ code is referenced.

---

## Redis Stream Write Flush Details
- Use `XADD` to add messages.
- Use `WAIT 1 0` after `XADD` to block until at least 1 replica confirms the write (guarantees flush to disk if AOF is enabled).
- If `WAIT` returns 0, fail the write.

---

## Example Redis Stream Publish (Go pseudocode)

```go
id, err := rdb.XAdd(ctx, &redis.XAddArgs{
    Stream: "db",
    Values: map[string]interface{}{ "envelope": jsonEnvelope },
}).Result()
if err != nil { ... }
replicas, err := rdb.Wait(ctx, 1, 0).Result()
if replicas < 1 {
    // fail write
}
return id // for later ack
```
