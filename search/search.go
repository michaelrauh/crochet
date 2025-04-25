package main

import (
	"context"
	"crochet/types"
	"log"
	"strings"
	"time"
)

// processWork handles the processing of work items, similar to the Elixir process_work function.
func processWork(
	ctx context.Context,
	state *State,
	workServerService types.WorkServerService,
	contextService types.ContextService,
	orthosService types.OrthosService,
	remediationsService types.RemediationsService,
) {
	for {
		// Pop a work item from the queue
		popResponse, err := workServerService.Pop(ctx)
		if err != nil {
			log.Printf("Error popping work item: %v", err)
			time.Sleep(5 * time.Second) // Retry after a delay
			continue
		}

		// Update the state based on the version
		versionResp, err := contextService.GetVersion(ctx)
		if err != nil {
			log.Printf("Error getting context version: %v", err)
			continue
		}

		if versionResp.Version != state.Version {
			contextResp, err := contextService.GetContext(ctx)
			if err != nil {
				log.Printf("Error getting context: %v", err)
				continue
			}

			// Update state with new version and vocabulary
			state.Version = contextResp.Version
			state.Vocabulary = contextResp.Vocabulary
			// Note: In a real implementation, you would also update the pairs
		}

		// Process the work item
		if popResponse.Status == "ok" && popResponse.Ortho != nil {
			top := popResponse.Ortho // This is of type *types.Ortho

			// Get forbidden and required values directly - capitalized to match ortho.go
			forbidden, required := GetRequirements(*top)

			// Convert forbidden to a map for filtering
			forbiddenMap := make(map[string]struct{})
			for _, word := range forbidden {
				forbiddenMap[word] = struct{}{}
			}

			// Filter the vocabulary to exclude forbidden words
			workingVocabulary := filterVocabulary(state.Vocabulary, forbiddenMap)

			// Generate candidates and remediations
			candidates, remediations := generateCandidatesAndRemediations(workingVocabulary, required, state.Pairs, *top)

			// Generate new orthos from candidates
			newOrthos := generateNewOrthos(candidates, *top)

			// Add new orthos to the database using SaveOrthos
			saveResponse, err := orthosService.SaveOrthos(ctx, newOrthos)
			if err != nil {
				log.Printf("Error saving orthos: %v", err)
				continue
			}
			log.Printf("Saved %d orthos", saveResponse.Count)

			// Add remediations to the database
			addResponse, err := remediationsService.AddRemediations(ctx, remediations)
			if err != nil {
				log.Printf("Error adding remediations: %v", err)
				continue
			}
			log.Printf("Added %d remediations", addResponse.Count)

			// Push new orthos to the work server
			pushResponse, err := workServerService.PushOrthos(ctx, newOrthos)
			if err != nil {
				log.Printf("Error pushing orthos to work server: %v", err)
				continue
			}
			log.Printf("Pushed %d orthos to work server", pushResponse.Count)

			// Acknowledge the work item
			_, err = workServerService.Ack(ctx, popResponse.ID)
			if err != nil {
				log.Printf("Error acknowledging work item: %v", err)
				continue
			}

			// Continue processing the next work item
			continue
		} else if popResponse.Status == "empty" {
			// If the queue is empty, retry after a delay
			log.Println("Queue is empty, retrying...")
			time.Sleep(5 * time.Second)
			continue
		}
	}
}

// filterVocabulary filters out forbidden words from the vocabulary.
func filterVocabulary(vocabulary []string, forbidden map[string]struct{}) []string {
	filtered := []string{}
	for _, word := range vocabulary {
		if _, exists := forbidden[word]; !exists {
			filtered = append(filtered, word)
		}
	}
	return filtered
}

// generateCandidatesAndRemediations generates candidates and remediations based on the vocabulary and requirements.
func generateCandidatesAndRemediations(
	workingVocabulary []string,
	required [][]string,
	pairs map[string]struct{},
	top types.Ortho,
) ([]string, []types.RemediationTuple) {
	candidates := []string{}
	remediations := []types.RemediationTuple{}

	for _, word := range workingVocabulary {
		missingRequired := findMissingRequired(required, pairs, word)
		if missingRequired != nil {
			remediations = append(remediations, types.RemediationTuple{
				Pair: append(missingRequired, word),
				Hash: generateUniqueHash(append(missingRequired, word)),
			})
		} else {
			candidates = append(candidates, word)
		}
	}

	return candidates, remediations
}

// findMissingRequired finds the first missing required pair for a given word.
func findMissingRequired(required [][]string, pairs map[string]struct{}, word string) []string {
	for _, req := range required {
		combined := append(req, word)
		key := strings.Join(combined, ",")
		if _, exists := pairs[key]; !exists {
			return req
		}
	}
	return nil
}

// generateNewOrthos generates new orthos from the candidates.
func generateNewOrthos(candidates []string, top types.Ortho) []types.Ortho {
	// Create a counter for generating new orthos
	counter := NewCounter()

	// Generate new orthos using the Add function - capitalized to match ortho.go
	var result []types.Ortho

	for _, word := range candidates {
		// Generate orthos using Add function
		newOrthos := Add(top, word, counter)
		result = append(result, newOrthos...)
	}

	return result
}

// generateUniqueHash generates a unique hash for a pair.
func generateUniqueHash(pair []string) string {
	return "hash-" + strings.Join(pair, "-")
}
