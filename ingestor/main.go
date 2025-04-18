package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"crochet/text"
)

type Corpus struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

type ContextService interface {
	SendMessage(message string) (map[string]interface{}, error)
}

type RealContextService struct {
	URL string
}

func (s *RealContextService) SendMessage(message string) (map[string]interface{}, error) {
	resp, err := http.Post(s.URL+"/input", "application/json", bytes.NewBuffer([]byte(message)))
	if err != nil {
		return nil, fmt.Errorf("error calling context service: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	log.Printf("Response from context service: %s", string(body))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("context service error: %s, Status Code: %d", string(body), resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		log.Printf("Error parsing context service response: %v", err)
		return nil, fmt.Errorf("invalid response from context service")
	}

	log.Printf("Parsed response: %v", response)
	return response, nil
}

func handleTextInput(w http.ResponseWriter, r *http.Request, contextService ContextService) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	var corpus Corpus
	if err := json.Unmarshal(body, &corpus); err != nil {
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	fmt.Printf("Title: %s\nText: %s\n", corpus.Title, corpus.Text)

	// Process the text using methods from crochet/text
	subphrases := text.GenerateSubphrases(corpus.Text) // Generate subphrases
	vocabulary := text.Vocabulary(corpus.Text)         // Generate vocabulary

	// Prepare the data to send to the context service
	contextInput := map[string]interface{}{
		"title":      corpus.Title,
		"vocabulary": vocabulary,
		"subphrases": subphrases,
	}

	contextInputJSON, err := json.Marshal(contextInput)
	if err != nil {
		log.Printf("Error preparing data for context service: %v", err)
		http.Error(w, "Error preparing data for context service", http.StatusInternalServerError)
		return
	}

	// Forward the processed data to the context service
	contextResponse, err := contextService.SendMessage(string(contextInputJSON))
	if err != nil {
		log.Printf("Error sending message to context service: %v", err)
		http.Error(w, "Error calling context service", http.StatusInternalServerError)
		return
	}

	// Respond to the client with the version from the context service
	response := map[string]interface{}{
		"status":  "success",
		"version": contextResponse["version"],
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func main() {
	port := os.Getenv("INGESTOR_PORT")
	if port == "" {
		panic("INGESTOR_PORT environment variable is not set")
	}

	contextServiceURL := os.Getenv("CONTEXT_SERVICE_URL")
	if contextServiceURL == "" {
		log.Fatal("CONTEXT_SERVICE_URL environment variable is not set")
	}
	contextService := &RealContextService{URL: contextServiceURL}

	http.HandleFunc("/ingest", func(w http.ResponseWriter, r *http.Request) {
		handleTextInput(w, r, contextService)
	})

	log.Printf("Server starting on port %s...\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
