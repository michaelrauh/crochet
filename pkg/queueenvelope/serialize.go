package queueenvelope

import (
	"crochet/pkg/ortho"
	"encoding/json"
)

// SerializeVocabulary takes a []string and returns a JSON envelope for vocabulary.
func SerializeVocabulary(words []string) ([]byte, error) {
	payload := VocabularyPayload{Words: words}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	env := Envelope{
		Type: EnvelopeTypeVocabulary,
		Data: data,
	}
	return json.Marshal(env)
}

// SerializeSubphrases takes a [][]string and returns a JSON envelope for subphrases.
func SerializeSubphrases(phrases [][]string) ([]byte, error) {
	payload := SubphrasesPayload{Phrases: phrases}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	env := Envelope{
		Type: EnvelopeTypeSubphrases,
		Data: data,
	}
	return json.Marshal(env)
}

// SerializeStartSigil returns a JSON envelope for a start sigil.
func SerializeStartSigil(sigil string) ([]byte, error) {
	payload := StartSigilPayload{Sigil: sigil}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	env := Envelope{
		Type: EnvelopeTypeStartSigil,
		Data: data,
	}
	return json.Marshal(env)
}

// SerializeEndSigil returns a JSON envelope for an end sigil.
func SerializeEndSigil(sigil string) ([]byte, error) {
	payload := EndSigilPayload{Sigil: sigil}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	env := Envelope{
		Type: EnvelopeTypeEndSigil,
		Data: data,
	}
	return json.Marshal(env)
}

func SerializeOrtho(o *ortho.Ortho) ([]byte, error) {
	data, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}

	env := Envelope{
		Type: EnvelopeTypeOrtho,
		Data: data,
	}
	return json.Marshal(env)
}
