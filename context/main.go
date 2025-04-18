package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

type Input struct {
	Message string `json:"message"`
}

func handleInput(w http.ResponseWriter, r *http.Request) {
	// Log the incoming request for debugging
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

	fmt.Printf("Received message: %s\n", input.Message)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func main() {
	log.Println("Starting context service...") // Debug log

	// Read the port from an environment variable
	port := os.Getenv("CONTEXT_PORT")
	if port == "" {
		panic("CONTEXT_PORT environment variable is not set")
	}

	host := "0.0.0.0" // Ensure the service binds to all interfaces
	http.HandleFunc("/input", handleInput)
	http.HandleFunc("/health", handleHealth) // Add health check endpoint

	log.Printf("Context service starting on %s:%s...\n", host, port)
	if err := http.ListenAndServe(host+":"+port, nil); err != nil {
		log.Fatalf("Context service failed to start: %v", err)
	}
}
