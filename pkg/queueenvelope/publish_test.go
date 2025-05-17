package queueenvelope

import (
	"context"
	"encoding/json"
	"testing"

	"crochet/pkg/rabbitmq"

	amqp091 "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
)

type mockChannel struct{}

// Consume implements rabbitmq.ChannelInterface.
func (m *mockChannel) Consume(queue string, consumer string, autoAck bool, exclusive bool, noLocal bool, noWait bool, args amqp091.Table) (<-chan amqp091.Delivery, error) {
	panic("unimplemented")
}

// Qos implements rabbitmq.ChannelInterface.
func (m *mockChannel) Qos(prefetchCount int, prefetchSize int, global bool) error {
	panic("unimplemented")
}

// Implement the missing Close method to satisfy rabbitmq.ChannelInterface
func (m *mockChannel) Close() error {
	return nil
}

func TestPublishVocabulary(t *testing.T) {
	var called bool
	var gotQueue string
	var gotBody []byte
	ctx := context.Background()
	var ch rabbitmq.ChannelInterface = &mockChannel{}

	mockPublish := func(ctx context.Context, ch rabbitmq.ChannelInterface, queue string, body []byte) error {
		called = true
		gotQueue = queue
		gotBody = body
		return nil
	}

	words := []string{"hello", "world"}
	err := PublishVocabulary(ctx, ch, words, mockPublish)
	assert.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, "db", gotQueue)

	var env Envelope
	err = json.Unmarshal(gotBody, &env)
	assert.NoError(t, err)
	assert.Equal(t, "Vocabulary", env.Type)

	var payload VocabularyPayload
	err = json.Unmarshal(env.Data, &payload)
	assert.NoError(t, err)
	assert.ElementsMatch(t, words, payload.Words)
}

func TestPublishSubphrases(t *testing.T) {
	var called bool
	var gotQueue string
	var gotBody []byte
	ctx := context.Background()
	var ch rabbitmq.ChannelInterface = &mockChannel{}

	mockPublish := func(ctx context.Context, ch rabbitmq.ChannelInterface, queue string, body []byte) error {
		called = true
		gotQueue = queue
		gotBody = body
		return nil
	}

	phrases := [][]string{{"hello", "world"}, {"foo", "bar"}}
	err := PublishSubphrases(ctx, ch, phrases, mockPublish)
	assert.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, "db", gotQueue)

	var env Envelope
	err = json.Unmarshal(gotBody, &env)
	assert.NoError(t, err)
	assert.Equal(t, "Subphrases", env.Type)

	var payload SubphrasesPayload
	err = json.Unmarshal(env.Data, &payload)
	assert.NoError(t, err)
	assert.Equal(t, phrases, payload.Phrases)
}

func TestPublishStartSigil(t *testing.T) {
	var called bool
	var gotQueue string
	var gotBody []byte
	ctx := context.Background()
	var ch rabbitmq.ChannelInterface = &mockChannel{}

	mockPublish := func(ctx context.Context, ch rabbitmq.ChannelInterface, queue string, body []byte) error {
		called = true
		gotQueue = queue
		gotBody = body
		return nil
	}

	sigil := "START"
	err := PublishStartSigil(ctx, ch, sigil, mockPublish)
	assert.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, "db", gotQueue)

	var env Envelope
	err = json.Unmarshal(gotBody, &env)
	assert.NoError(t, err)
	assert.Equal(t, "StartSigil", env.Type)

	var payload StartSigilPayload
	err = json.Unmarshal(env.Data, &payload)
	assert.NoError(t, err)
	assert.Equal(t, sigil, payload.Sigil)
}

func TestPublishEndSigil(t *testing.T) {
	var called bool
	var gotQueue string
	var gotBody []byte
	ctx := context.Background()
	var ch rabbitmq.ChannelInterface = &mockChannel{}

	mockPublish := func(ctx context.Context, ch rabbitmq.ChannelInterface, queue string, body []byte) error {
		called = true
		gotQueue = queue
		gotBody = body
		return nil
	}

	sigil := "END"
	err := PublishEndSigil(ctx, ch, sigil, mockPublish)
	assert.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, "db", gotQueue)

	var env Envelope
	err = json.Unmarshal(gotBody, &env)
	assert.NoError(t, err)
	assert.Equal(t, "EndSigil", env.Type)

	var payload EndSigilPayload
	err = json.Unmarshal(env.Data, &payload)
	assert.NoError(t, err)
	assert.Equal(t, sigil, payload.Sigil)
}
