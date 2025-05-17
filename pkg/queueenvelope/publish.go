package queueenvelope

import (
	"context"
	"crochet/pkg/ortho"
	"crochet/pkg/rabbitmq"
	"log"
)

// PublishFunc matches the signature of rabbitmq.Queue.Publish.
type PublishFunc func(ctx context.Context, ch rabbitmq.ChannelInterface, queueName string, body []byte) error

func PublishVocabulary(ctx context.Context, ch rabbitmq.ChannelInterface, words []string, publish PublishFunc) error {
	log.Printf("[QueueEnvelope] Starting to serialize vocabulary: %v", words)
	body, err := SerializeVocabulary(words)
	if err != nil {
		log.Printf("[QueueEnvelope] Error serializing vocabulary: %v", err)
		return err
	}
	log.Printf("[QueueEnvelope] Serialized vocabulary, publishing to queue db: %s", string(body))
	err = publish(ctx, ch, "db", body)
	if err != nil {
		log.Printf("[QueueEnvelope] Error publishing vocabulary: %v", err)
	} else {
		log.Printf("[QueueEnvelope] Successfully published vocabulary")
	}
	return err
}

func PublishSubphrases(ctx context.Context, ch rabbitmq.ChannelInterface, phrases [][]string, publish PublishFunc) error {
	log.Printf("[QueueEnvelope] Starting to serialize subphrases: %v", phrases)
	body, err := SerializeSubphrases(phrases)
	if err != nil {
		log.Printf("[QueueEnvelope] Error serializing subphrases: %v", err)
		return err
	}
	log.Printf("[QueueEnvelope] Serialized subphrases, publishing to queue db: %s", string(body))
	err = publish(ctx, ch, "db", body)
	if err != nil {
		log.Printf("[QueueEnvelope] Error publishing subphrases: %v", err)
	} else {
		log.Printf("[QueueEnvelope] Successfully published subphrases")
	}
	return err
}

func PublishStartSigil(ctx context.Context, ch rabbitmq.ChannelInterface, sigil string, publish PublishFunc) error {
	log.Printf("[QueueEnvelope] Starting to serialize start sigil: %s", sigil)
	body, err := SerializeStartSigil(sigil)
	if err != nil {
		log.Printf("[QueueEnvelope] Error serializing start sigil: %v", err)
		return err
	}
	log.Printf("[QueueEnvelope] Serialized start sigil, publishing to queue db: %s", string(body))
	err = publish(ctx, ch, "db", body)
	if err != nil {
		log.Printf("[QueueEnvelope] Error publishing start sigil: %v", err)
	} else {
		log.Printf("[QueueEnvelope] Successfully published start sigil")
	}
	return err
}

func PublishEndSigil(ctx context.Context, ch rabbitmq.ChannelInterface, sigil string, publish PublishFunc) error {
	log.Printf("[QueueEnvelope] Starting to serialize end sigil: %s", sigil)
	body, err := SerializeEndSigil(sigil)
	if err != nil {
		log.Printf("[QueueEnvelope] Error serializing end sigil: %v", err)
		return err
	}
	log.Printf("[QueueEnvelope] Serialized end sigil, publishing to queue db: %s", string(body))
	err = publish(ctx, ch, "db", body)
	if err != nil {
		log.Printf("[QueueEnvelope] Error publishing end sigil: %v", err)
	} else {
		log.Printf("[QueueEnvelope] Successfully published end sigil")
	}
	return err
}

func PublishOrtho(ctx context.Context, ch rabbitmq.ChannelInterface, o *ortho.Ortho, publish PublishFunc) error {
	log.Printf("[QueueEnvelope] Starting to serialize ortho")
	body, err := SerializeOrtho(o)
	if err != nil {
		log.Printf("[QueueEnvelope] Error serializing ortho: %v", err)
		return err
	}
	log.Printf("[QueueEnvelope] Serialized ortho, publishing to queue db: %s", string(body))
	err = publish(ctx, ch, "db", body)
	if err != nil {
		log.Printf("[QueueEnvelope] Error publishing ortho: %v", err)
	} else {
		log.Printf("[QueueEnvelope] Successfully published ortho")
	}
	return err
}
