package rabbitmq

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rabbitmq/amqp091-go"
)

type mockChannel struct {
	amqp091.Channel
	msgs   chan amqp091.Delivery
	closed bool
}

func (m *mockChannel) Close() error {
	m.closed = true
	return nil
}

func (m *mockChannel) Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp091.Table) (<-chan amqp091.Delivery, error) {
	return m.msgs, nil
}

func (m *mockChannel) Qos(prefetchCount, prefetchSize int, global bool) error {
	// Do nothing, just mock
	return nil
}

var _ ChannelInterface = (*mockChannel)(nil)

var _ = Describe("Queue.ConsumeBatch", func() {
	It("should consume up to max messages and ack them individually", func() {
		msgs := make(chan amqp091.Delivery, 10)
		for i := 0; i < 5; i++ {
			msgs <- amqp091.Delivery{DeliveryTag: uint64(i + 1)}
		}
		close(msgs)
		ch := &mockChannel{msgs: msgs}
		q := &queue{}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		batch, err := q.consumeBatchWithQos(ctx, ch, "test-queue", 10, func(_ ChannelInterface, _ int) error { return nil })
		Expect(err).NotTo(HaveOccurred())
		Expect(len(batch)).To(Equal(5))
		for i, d := range batch {
			Expect(d.DeliveryTag).To(Equal(uint64(i + 1)))
		}
	})
})

func TestRabbitMQ(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RabbitMQ Suite")
}
