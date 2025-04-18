package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type MockContextService struct {
	SendMessageFunc func(message string) error
}

func (m *MockContextService) SendMessage(message string) error {
	return m.SendMessageFunc(message)
}

func TestHandleTextInputValidJSON(t *testing.T) {
	mockService := &MockContextService{
		SendMessageFunc: func(message string) error {
			return nil
		},
	}

	body := `{"title": "Test Title", "text": "Test Content"}`
	req := httptest.NewRequest(http.MethodPost, "/ingest", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handleTextInput(w, req, mockService)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", resp.StatusCode, http.StatusOK)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Errorf("json.Unmarshal failed for actual response: %v", err)
	}

	if response["status"] != "success" {
		t.Errorf("unexpected response: got %v want %v", response["status"], "success")
	}
}

func TestHandleTextInputInvalidJSON(t *testing.T) {
	mockService := &MockContextService{
		SendMessageFunc: func(message string) error {
			return nil
		},
	}

	invalidJSON := `{title:"Invalid JSON"}`
	req := httptest.NewRequest(http.MethodPost, "/ingest", bytes.NewBufferString(invalidJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handleTextInput(w, req, mockService)

	if w.Code != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code for invalid JSON: got %v want %v", w.Code, http.StatusBadRequest)
	}

	if !strings.Contains(w.Body.String(), "Invalid JSON format") {
		t.Errorf("handler returned unexpected error message: got %v want to contain %v", w.Body.String(), "Invalid JSON format")
	}
}

func TestHandleTextInputEmptyBody(t *testing.T) {
	mockService := &MockContextService{
		SendMessageFunc: func(message string) error {
			return nil
		},
	}

	req, err := http.NewRequest("POST", "/ingest", bytes.NewBufferString(""))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()

	handleTextInput(rr, req, mockService)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code for empty body: got %v want %v", status, http.StatusBadRequest)
	}
}

func TestHandleTextInputWrongMethod(t *testing.T) {
	mockService := &MockContextService{
		SendMessageFunc: func(message string) error {
			return nil
		},
	}

	req, err := http.NewRequest("GET", "/ingest", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()

	handleTextInput(rr, req, mockService)

	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("handler returned wrong status code for wrong method: got %v want %v",
			status, http.StatusMethodNotAllowed)
	}

	if !strings.Contains(rr.Body.String(), "Method not allowed") {
		t.Errorf("handler returned unexpected error message: got %v want to contain %v",
			rr.Body.String(), "Method not allowed")
	}
}
