package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractVocabulary(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected any
	}{
		{
			name:     "Empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "Single word",
			input:    "hello",
			expected: []string{"hello"},
		},
		{
			name:     "Multiple words",
			input:    "hello world",
			expected: []string{"hello", "world"},
		},
		{
			name:     "Repeated words",
			input:    "hello world hello",
			expected: []string{"hello", "world"},
		},
		{
			name:     "mixed case",
			input:    "HelLo world hello",
			expected: []string{"hello", "world"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractVocabulary(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractSubphrases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected any
	}{
		{
			name:     "Empty string",
			input:    "",
			expected: [][]string{},
		},
		{
			name:     "Single word",
			input:    "hello",
			expected: [][]string{},
		},
		{
			name:     "Multiple words",
			input:    "hello world",
			expected: [][]string{{"hello", "world"}},
		},
		{
			name:     "mixed case",
			input:    "HelLo world",
			expected: [][]string{{"hello", "world"}},
		},
		{
			name:     "strip punctuation",
			input:    "Hello, world",
			expected: [][]string{{"hello", "world"}},
		},
		{
			name:     "split by punctuation",
			input:    "Hello world. Hello jupiter! Hello saturn; Hello Venus.",
			expected: [][]string{{"hello", "world"}, {"hello", "jupiter"}, {"hello", "saturn"}, {"hello", "venus"}},
		},
		{
			name:     "find subphrases of length 2 or greater",
			input:    "Hello world good morning.",
			expected: [][]string{{"hello", "world", "good", "morning"}, {"hello", "world", "good"}, {"world", "good", "morning"}, {"hello", "world"}, {"world", "good"}, {"good", "morning"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractSubphrases(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
