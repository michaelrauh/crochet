package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type Corpus struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

func handleTextInput(w http.ResponseWriter, r *http.Request) {
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func main() {
	port := "8080"

	http.HandleFunc("/ingest", handleTextInput)

	log.Printf("Server starting on port %s...\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
