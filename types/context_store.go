package types

import (
	"strings"
)

// InitContextStore initializes a new context memory store
func InitContextStore() *ContextMemoryStore {
	return &ContextMemoryStore{
		Vocabulary: make(map[string]struct{}),
		Subphrases: make(map[string]struct{}),
	}
}

// SaveVocabularyToStore saves vocabulary to the store and returns newly added items
func SaveVocabularyToStore(store *ContextMemoryStore, vocabulary []string) []string {
	var newlyAdded []string
	for _, word := range vocabulary {
		if _, exists := store.Vocabulary[word]; !exists {
			store.Vocabulary[word] = struct{}{}
			newlyAdded = append(newlyAdded, word)
		}
	}
	return newlyAdded
}

// SaveSubphrasesToStore saves subphrases to the store and returns newly added items
func SaveSubphrasesToStore(store *ContextMemoryStore, subphrases [][]string) [][]string {
	var newlyAdded [][]string
	for _, subphrase := range subphrases {
		joinedSubphrase := strings.Join(subphrase, " ")
		if _, exists := store.Subphrases[joinedSubphrase]; !exists {
			store.Subphrases[joinedSubphrase] = struct{}{}
			newlyAdded = append(newlyAdded, subphrase)
		}
	}
	return newlyAdded
}

// SplitSubphrase splits a joined subphrase string back into individual words
func SplitSubphrase(joinedSubphrase string) []string {
	// Simple split by space since we join with space in SaveSubphrasesToStore
	return strings.Split(joinedSubphrase, " ")
}
