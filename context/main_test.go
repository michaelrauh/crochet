package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMain(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("main function panicked: %v", r)
		}
	}()

	// Don't actually run main in tests - just ensure it compiles
	// Instead, we'll test specific functions
}

func TestVersionCounter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	initStore() // Reset the store and version counter

	router.POST("/input", ginHandleInput)

	// First request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/input", strings.NewReader(`{"vocabulary": ["word1"], "subphrases": [["word1", "word2"]]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

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
	req, _ = http.NewRequest(http.MethodPost, "/input", strings.NewReader(`{"vocabulary": ["word1"], "subphrases": [["word1", "word2"]]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

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

func TestGetVersion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	initStore() // Reset the store and version counter

	router.GET("/version", ginGetVersion)

	// Test the version endpoint
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/version", nil)
	router.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status OK, got %v", resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// The version should be 1 (initial value)
	if response["version"] != float64(1) {
		t.Errorf("expected version 1, got %v", response["version"])
	}

	// Let's change the versionCounter and test again
	versionCounter = 42

	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/version", nil)
	router.ServeHTTP(w, req)

	resp = w.Result()
	var secondResponse map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&secondResponse); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if secondResponse["version"] != float64(42) {
		t.Errorf("expected version 42, got %v", secondResponse["version"])
	}
}
