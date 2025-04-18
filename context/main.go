package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

type Input struct {
	Vocabulary []string   `json:"vocabulary"`
	Subphrases [][]string `json:"subphrases"`
}

type MemoryStore struct {
	Vocabulary map[string]struct{}
	Subphrases map[string]struct{}
}

var store MemoryStore
var versionCounter int

func initStore() {
	store = MemoryStore{
		Vocabulary: make(map[string]struct{}),
		Subphrases: make(map[string]struct{}),
	}
	versionCounter = 1
	fmt.Println("In-memory store initialized successfully")
}

func saveVocabularyToStore(vocabulary []string) ([]string, error) {
	var newlyAdded []string
	for _, word := range vocabulary {
		if _, exists := store.Vocabulary[word]; !exists {
			store.Vocabulary[word] = struct{}{}
			newlyAdded = append(newlyAdded, word)
		}
	}
	return newlyAdded, nil
}

func saveSubphrasesToStore(subphrases [][]string) ([][]string, error) {
	var newlyAdded [][]string
	for _, subphrase := range subphrases {
		joinedSubphrase := strings.Join(subphrase, " ")
		if _, exists := store.Subphrases[joinedSubphrase]; !exists {
			store.Subphrases[joinedSubphrase] = struct{}{}
			newlyAdded = append(newlyAdded, subphrase) // Keep subphrases as nested lists
		}
	}
	return newlyAdded, nil
}

func handleInput(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received request: %v", r)

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

	var input Input
	if err := json.Unmarshal(body, &input); err != nil {
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	newVocabulary, err := saveVocabularyToStore(input.Vocabulary)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to save vocabulary: %v", err), http.StatusInternalServerError)
		return
	}

	newSubphrases, err := saveSubphrasesToStore(input.Subphrases)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to save subphrases: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"newVocabulary": newVocabulary,
		"newSubphrases": newSubphrases,  // Keep subphrases as nested lists
		"version":       versionCounter, // Include version in the response
	}
	versionCounter++                                         // Increment the version counter
	log.Printf("Sending response to ingestor: %v", response) // Log the response explicitly
	log.Println("Flushing logs to ensure visibility")        // Explicit log flushing

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	log.Println("Health check endpoint called")
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("OK"))
	if err != nil {
		log.Printf("Error writing health check response: %v", err)
	}
}

func main() {
	log.Println("Starting context service...")

	port := os.Getenv("CONTEXT_PORT")
	if port == "" {
		panic("CONTEXT_PORT environment variable is not set")
	}

	host := os.Getenv("CONTEXT_HOST")
	if host == "" {
		panic("CONTEXT_HOST environment variable is not set")
	}

	initStore()

	http.HandleFunc("/input", handleInput)
	http.HandleFunc("/health", handleHealth)

	log.Printf("Context service starting on %s:%s...\n", host, port)
	if err := http.ListenAndServe(host+":"+port, nil); err != nil {
		log.Fatalf("Context service failed to start: %v", err)
	}

	log.Println("Context service is ready to accept requests")
}
