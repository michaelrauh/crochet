package queueenvelope

import (
	"context"
	"crochet/pkg/ortho"
	"crochet/pkg/redisstream"
	"log"
)

func PublishVocabulary(ctx context.Context, queue redisstream.Queue, words []string) error {
	jsonEnvelope, err := SerializeVocabulary(words)
	if err != nil {
		return err
	}
	values := map[string]interface{}{"envelope": string(jsonEnvelope)}
	return queue.PublishBatch(ctx, "db", []map[string]interface{}{values})
}

func PublishSubphrases(ctx context.Context, queue redisstream.Queue, phrases [][]string) error {
	jsonEnvelope, err := SerializeSubphrases(phrases)
	if err != nil {
		return err
	}
	values := map[string]interface{}{"envelope": string(jsonEnvelope)}
	return queue.PublishBatch(ctx, "db", []map[string]interface{}{values})
}

func PublishStartSigil(ctx context.Context, queue redisstream.Queue, sigil string) error {
	jsonEnvelope, err := SerializeStartSigil(sigil)
	if err != nil {
		return err
	}
	values := map[string]interface{}{"envelope": string(jsonEnvelope)}
	return queue.PublishBatch(ctx, "db", []map[string]interface{}{values})
}

func PublishEndSigil(ctx context.Context, queue redisstream.Queue, sigil string) error {
	jsonEnvelope, err := SerializeEndSigil(sigil)
	if err != nil {
		return err
	}
	values := map[string]interface{}{"envelope": string(jsonEnvelope)}
	return queue.PublishBatch(ctx, "db", []map[string]interface{}{values})
}

func PublishOrtho(ctx context.Context, queue redisstream.Queue, o *ortho.Ortho) error {
	jsonEnvelope, err := SerializeOrtho(o)
	if err != nil {
		log.Printf("Failed to serialize ortho envelope: %v", err)
		return err
	}
	values := map[string]interface{}{"envelope": string(jsonEnvelope)}
	return queue.PublishBatch(ctx, "db", []map[string]interface{}{values})
}
