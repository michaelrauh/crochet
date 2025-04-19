package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

type RemediationRequest struct {
	Pairs [][]string `json:"pairs"`
}

func okHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "OK"})
}

func remediateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request RemediationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Printf("Error decoding request: %v", err)
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	log.Printf("Received %d pairs for remediation", len(request.Pairs))

	// Log the pairs we received
	for i, pair := range request.Pairs {
		log.Printf("Pair %d: %v", i+1, pair)
	}

	// Return a list of mock hashes as the response
	hashes := []string{
		"1234567890abcdef1234567890abcdef",
		"abcdef1234567890abcdef1234567890",
		"aabbccddeeff00112233445566778899",
		"99887766554433221100ffeeddccbbaa",
		"112233445566778899aabbccddeeff00",
		"00ffeeddccbbaa99887766554433221",
	}

	response := map[string]interface{}{
		"status": "OK",
		"hashes": hashes,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func main() {
	port := os.Getenv("REMEDIATIONS_PORT")
	if port == "" {
		panic("REMEDIATIONS_PORT environment variable must be set")
	}

	http.HandleFunc("/", okHandler)
	http.HandleFunc("/remediate", remediateHandler)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Remediations service starting on port %s...", port)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
