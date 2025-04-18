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
	SendMessage(message string) error
}

type RealContextService struct {
	URL string
}

func (s *RealContextService) SendMessage(message string) error {
	contextInput := map[string]string{"message": message}
	contextInputJSON, err := json.Marshal(contextInput)
	if err != nil {
		return fmt.Errorf("error preparing data for context service: %w", err)
	}

	resp, err := http.Post(s.URL, "application/json", bytes.NewBuffer(contextInputJSON))
	if err != nil {
		return fmt.Errorf("error calling context service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("context service error: %s, Status Code: %d", string(body), resp.StatusCode)
	}

	return nil
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

	subphrases := text.GenerateSubphrases(corpus.Text)
	vocabulary := text.Vocabulary(corpus.Text)

	message := fmt.Sprintf("Title: %s, Subphrases: %v, Vocabulary: %v", corpus.Title, subphrases, vocabulary)
	if err := contextService.SendMessage(message); err != nil {
		log.Printf("Error sending message to context service: %v", err)
		http.Error(w, "Error calling context service", http.StatusInternalServerError)
		return
	}

	response := map[string]string{"status": "success"}
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
