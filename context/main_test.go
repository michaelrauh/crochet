package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMain(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("main function panicked: %v", r)
		}
	}()

	main()
}

func TestVersionCounter(t *testing.T) {
	initStore() // Reset the store and version counter

	req := httptest.NewRequest(http.MethodPost, "/input", strings.NewReader(`{"vocabulary": ["word1"], "subphrases": [["word1", "word2"]]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handleInput(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status OK, got %v", resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["version"] != float64(1) { // JSON unmarshals numbers as float64
		t.Errorf("expected version 1, got %v", response["version"])
	}

	// Make another request to verify version increments
	w = httptest.NewRecorder()
	handleInput(w, req)

	resp = w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status OK, got %v", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["version"] != float64(2) {
		t.Errorf("expected version 2, got %v", response["version"])
	}
}
