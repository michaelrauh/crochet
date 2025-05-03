package text

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestGenerateSubphrases_EmptyString(t *testing.T) {
	input := ""
	expected := [][]string{}
	result := GenerateSubphrases(input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GenerateSubphrases(%q) = %v, want %v", input, result, expected)
	}
}

func TestGenerateSubphrases_SingleWord(t *testing.T) {
	input := "hello"
	expected := [][]string{}
	result := GenerateSubphrases(input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GenerateSubphrases(%q) = %v, want %v", input, result, expected)
	}
}

func TestGenerateSubphrases_SingleSentence(t *testing.T) {
	input := "hello world"
	expected := [][]string{
		{"hello", "world"},
	}
	result := GenerateSubphrases(input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GenerateSubphrases(%q) = %v, want %v", input, result, expected)
	}
}

func TestGenerateSubphrases_MultipleWords(t *testing.T) {
	input := "hello beautiful world"
	expected := [][]string{
		{"hello", "beautiful"},
		{"beautiful", "world"},
		{"hello", "beautiful", "world"},
	}
	result := GenerateSubphrases(input)
	sortSubphrasesList(result)
	sortSubphrasesList(expected)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GenerateSubphrases(%q) = %v, want %v", input, result, expected)
	}
}

func TestGenerateSubphrases_MultipleSentences(t *testing.T) {
	input := "hello world. goodbye moon"
	expected := [][]string{
		{"hello", "world"},
		{"goodbye", "moon"},
	}
	result := GenerateSubphrases(input)
	sortSubphrasesList(result)
	sortSubphrasesList(expected)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GenerateSubphrases(%q) = %v, want %v", input, result, expected)
	}
}

func TestGenerateSubphrases_DuplicateSubphrases(t *testing.T) {
	input := "hello world hello world"
	expected := [][]string{
		{"hello", "world"},
		{"world", "hello"},
		{"hello", "world", "hello"},
		{"world", "hello", "world"},
		{"hello", "world", "hello", "world"},
	}
	result := GenerateSubphrases(input)
	sortSubphrasesList(result)
	sortSubphrasesList(expected)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GenerateSubphrases(%q) = %v, want %v", input, result, expected)
	}
}

func sortSubphrasesList(phrases [][]string) {
	sort.Slice(phrases, func(i, j int) bool {
		iStr := strings.Join(phrases[i], " ")
		jStr := strings.Join(phrases[j], " ")
		return iStr < jStr
	})
}

func TestVocabulary_EmptyString(t *testing.T) {
	input := ""
	expected := []string{}
	result := Vocabulary(input)
	sort.Strings(result)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Vocabulary(%q) = %v, want %v", input, result, expected)
	}
}

func TestVocabulary_SingleWord(t *testing.T) {
	input := "hello"
	expected := []string{"hello"}
	result := Vocabulary(input)
	sort.Strings(result)
	sort.Strings(expected)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Vocabulary(%q) = %v, want %v", input, result, expected)
	}
}

func TestVocabulary_MultipleSentences(t *testing.T) {
	input := "hello world. hello moon. goodbye world"
	expected := []string{"goodbye", "hello", "moon", "world"}
	result := Vocabulary(input)
	sort.Strings(result)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Vocabulary(%q) = %v, want %v", input, result, expected)
	}
}
