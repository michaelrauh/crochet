package main

import (
	"sort"
	"strings"
	"unicode"
)

func extractVocabulary(input string) []string {
	words := strings.Fields(input)
	vocabulary := make(map[string]struct{})
	for _, word := range words {
		normalized := strings.ToLower(word)
		vocabulary[normalized] = struct{}{}
	}
	result := make([]string, 0)
	for word := range vocabulary {
		result = append(result, word)
	}
	sort.Strings(result)
	return result
}

func extractSubphrases(input string) [][]string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return [][]string{}
	}
	sentences := splitSentences(trimmed)
	result := make([][]string, 0)
	for _, sentence := range sentences {
		words := cleanWords(sentence)
		if len(words) < 2 {
			continue
		}
		result = append(result, extractAllSubphrases(words)...)
	}
	return result
}

func splitSentences(text string) []string {
	splitFunc := func(r rune) bool {
		return r == '.' || r == '!' || r == ';' || r == '?'
	}
	raw := strings.FieldsFunc(text, splitFunc)
	sentences := make([]string, 0, len(raw))
	for _, s := range raw {
		s = strings.TrimSpace(s)
		if s != "" {
			sentences = append(sentences, s)
		}
	}
	return sentences
}

func cleanWords(sentence string) []string {
	words := strings.Fields(sentence)
	cleaned := make([]string, 0, len(words))
	for _, w := range words {
		clean := strings.Map(func(r rune) rune {
			if unicode.IsPunct(r) {
				return -1
			}
			return r
		}, w)
		clean = strings.ToLower(clean)
		if clean != "" {
			cleaned = append(cleaned, clean)
		}
	}
	return cleaned
}

func extractAllSubphrases(words []string) [][]string {
	n := len(words)
	subphrases := make([][]string, 0)
	for length := n; length >= 2; length-- {
		for start := 0; start <= n-length; start++ {
			sub := words[start : start+length]
			subphrases = append(subphrases, append([]string{}, sub...))
		}
	}
	return subphrases
}
