package text

import (
	"regexp"
	"strings"
)

func splitBySentence(s string) []string {
	if s == "" {
		return []string{}
	}

	re := regexp.MustCompile(`[?;.\n]+`)

	sentences := re.Split(s, -1)

	result := make([]string, 0, len(sentences))
	for _, sentence := range sentences {
		trimmed := strings.TrimSpace(sentence)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

func splitBySentenceAndWords(s string) [][]string {
	sentences := splitBySentence(s)

	result := make([][]string, 0, len(sentences))
	for _, sentence := range sentences {
		words := strings.Fields(sentence)
		result = append(result, words)
	}

	return result
}

func GenerateSubphrases(input string) [][]string {
	sentences := splitBySentenceAndWords(input)

	result := [][]string{}

	uniqueSubphrases := make(map[string]bool)

	for _, words := range sentences {

		if len(words) < 2 {
			continue
		}

		for start := 0; start < len(words); start++ {
			for end := start + 2; end <= len(words); end++ {
				subphrase := words[start:end]
				key := strings.Join(subphrase, " ")

				if !uniqueSubphrases[key] {
					uniqueSubphrases[key] = true
					result = append(result, subphrase)
				}
			}
		}
	}

	return result
}

func Vocabulary(input string) []string {
	if input == "" {
		return []string{}
	}

	sentences := splitBySentenceAndWords(input)

	uniqueWords := make(map[string]bool)

	for _, sentence := range sentences {
		for _, word := range sentence {
			uniqueWords[word] = true
		}
	}

	result := make([]string, 0, len(uniqueWords))
	for word := range uniqueWords {
		result = append(result, word)
	}

	return result
}
