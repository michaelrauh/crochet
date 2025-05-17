package queueenvelope

import "encoding/json"

type Envelope struct {
	Type string          `json:"Type"`
	Data json.RawMessage `json:"Data"`
}

type VocabularyPayload struct {
	Words []string `json:"Words"`
}

type SubphrasesPayload struct {
	Phrases [][]string `json:"Phrases"`
}

type StartSigilPayload struct {
	Sigil string `json:"Sigil"`
}

type EndSigilPayload struct {
	Sigil string `json:"Sigil"`
}
