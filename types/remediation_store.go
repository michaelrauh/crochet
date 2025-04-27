// filepath: /Users/michaelrauh/dev/crochet/types/remediation_store.go
package types

// InitRemediationStore initializes a new RemediationMemoryStore with an empty slice
func InitRemediationStore() *RemediationMemoryStore {
	return &RemediationMemoryStore{
		Remediations: make([]RemediationTuple, 0),
	}
}

// SaveRemediationsToStore adds remediations to the store and returns the number of new items added
func SaveRemediationsToStore(store *RemediationMemoryStore, remediations []RemediationTuple) int {
	// Create a map for faster lookups
	existingMap := make(map[string]struct{})

	// Populate the map with existing remediations
	for _, remediation := range store.Remediations {
		// Create a string key from the pair for map lookup
		pairKey := createPairKey(remediation.Pair)
		existingMap[pairKey] = struct{}{}
	}

	addedCount := 0
	for _, newRemediation := range remediations {
		// Create a pair key for the new remediation
		pairKey := createPairKey(newRemediation.Pair)

		// Check if this pair already exists
		if _, exists := existingMap[pairKey]; exists {
			// This remediation already exists
			continue
		}

		// Add the new remediation
		store.Remediations = append(store.Remediations, newRemediation)
		// Update our map to include this new remediation
		existingMap[pairKey] = struct{}{}
		addedCount++
	}

	return addedCount
}

// createPairKey creates a string key from a string slice for map lookups
// This is more efficient than comparing slices directly
func createPairKey(pair []string) string {
	if len(pair) == 0 {
		return ""
	}
	result := pair[0]
	for i := 1; i < len(pair); i++ {
		result += ":" + pair[i]
	}
	return result
}

// CompareStringSlices compares two string slices for equality
// Exported so it can be used by other packages
func CompareStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}
