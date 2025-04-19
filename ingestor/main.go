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

type RemediationsService interface {
	FetchRemediations(subphrases [][]string) (map[string]interface{}, error)
}

type RealRemediationsService struct {
	URL string
}

func (s *RealRemediationsService) FetchRemediations(subphrases [][]string) (map[string]interface{}, error) {
	// Filter subphrases to only include pairs
	var pairs [][]string
	for _, subphrase := range subphrases {
		if len(subphrase) == 2 {
			pairs = append(pairs, subphrase)
		}
	}

	// Prepare request payload
	requestData := map[string]interface{}{
		"pairs": pairs,
	}

	requestJSON, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("error marshaling remediations request: %w", err)
	}

	// Send request to remediations service
	resp, err := http.Post(s.URL+"/remediate", "application/json", bytes.NewBuffer(requestJSON))
	if err != nil {
		return nil, fmt.Errorf("error calling remediations service: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	log.Printf("Response from remediations service: %s", string(body))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remediations service error: %s, Status Code: %d", string(body), resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		log.Printf("Error parsing remediations service response: %v", err)
		return nil, fmt.Errorf("invalid response from remediations service")
	}

	return response, nil
}

func handleTextInput(w http.ResponseWriter, r *http.Request, contextService ContextService, remediationsService RemediationsService) {
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

	// Extract newSubphrases from the context service response
	newSubphrases, ok := contextResponse["newSubphrases"].([]interface{})
	if !ok {
		log.Printf("Error: newSubphrases not found in context response or has unexpected type")
		http.Error(w, "Invalid response from context service", http.StatusInternalServerError)
		return
	}

	// Convert newSubphrases to the expected format for remediations service
	var subphrasesForRemediations [][]string
	for _, subphrase := range newSubphrases {
		if subphraseArray, ok := subphrase.([]interface{}); ok {
			var stringArray []string
			for _, word := range subphraseArray {
				if str, ok := word.(string); ok {
					stringArray = append(stringArray, str)
				}
			}
			subphrasesForRemediations = append(subphrasesForRemediations, stringArray)
		}
	}

	// Call remediations service with the filtered subphrases
	remediationsResponse, err := remediationsService.FetchRemediations(subphrasesForRemediations)
	if err != nil {
		log.Printf("Error fetching remediations: %v", err)
		// Continue without remediations for now
	} else {
		log.Printf("Remediations response: %v", remediationsResponse)
		// For now, we'll just log the response, not returning it to the client
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

	remediationsServiceURL := os.Getenv("REMEDIATIONS_SERVICE_URL")
	if remediationsServiceURL == "" {
		log.Fatal("REMEDIATIONS_SERVICE_URL environment variable is not set")
	}
	remediationsService := &RealRemediationsService{URL: remediationsServiceURL}

	http.HandleFunc("/ingest", func(w http.ResponseWriter, r *http.Request) {
		handleTextInput(w, r, contextService, remediationsService)
	})

	log.Printf("Server starting on port %s...\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
