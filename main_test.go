package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleTextInputValidJSON(t *testing.T) {
	validJSON := `{"title":"Test Title","text":"Test Content"}`
	req, err := http.NewRequest("POST", "/ingest", bytes.NewBufferString(validJSON))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleTextInput)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var response map[string]string
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Fatal(err)
	}
	if response["status"] != "success" {
		t.Errorf("handler returned unexpected body: got %v want %v", response["status"], "success")
	}
}

func TestHandleTextInputInvalidJSON(t *testing.T) {
	invalidJSON := `{title:"Invalid JSON"}`
	req, err := http.NewRequest("POST", "/ingest", bytes.NewBufferString(invalidJSON))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleTextInput)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code for invalid JSON: got %v want %v", status, http.StatusBadRequest)
	}

	if !strings.Contains(rr.Body.String(), "Invalid JSON format") {
		t.Errorf("handler returned unexpected error message: got %v want to contain %v", rr.Body.String(), "Invalid JSON format")
	}
}

func TestHandleTextInputEmptyBody(t *testing.T) {
	req, err := http.NewRequest("POST", "/ingest", bytes.NewBufferString(""))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleTextInput)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code for empty body: got %v want %v", status, http.StatusBadRequest)
	}
}

func TestHandleTextInputWrongMethod(t *testing.T) {
	req, err := http.NewRequest("GET", "/ingest", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleTextInput)

	handler.ServeHTTP(rr, req)

	// Now main.go checks for HTTP method, so we should get StatusMethodNotAllowed
	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("handler returned wrong status code for wrong method: got %v want %v",
			status, http.StatusMethodNotAllowed)
	}

	// Check that error message contains "Method not allowed"
	if !strings.Contains(rr.Body.String(), "Method not allowed") {
		t.Errorf("handler returned unexpected error message: got %v want to contain %v",
			rr.Body.String(), "Method not allowed")
	}
}
