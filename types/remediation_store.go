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
	existingMap := make(map[string]map[string]struct{})

	// Populate the map with existing remediations
	// Using a nested map: hash -> pairString -> struct{}
	for _, remediation := range store.Remediations {
		if _, exists := existingMap[remediation.Hash]; !exists {
			existingMap[remediation.Hash] = make(map[string]struct{})
		}
		// Create a string key from the pair for map lookup
		pairKey := createPairKey(remediation.Pair)
		existingMap[remediation.Hash][pairKey] = struct{}{}
	}

	addedCount := 0
	for _, newRemediation := range remediations {
		// Check if the hash exists in our map
		if hashMap, hashExists := existingMap[newRemediation.Hash]; hashExists {
			// Check if the pair exists for this hash
			pairKey := createPairKey(newRemediation.Pair)
			if _, pairExists := hashMap[pairKey]; pairExists {
				// This remediation already exists
				continue
			}
		} else {
			// Create a new entry for this hash if it doesn't exist
			existingMap[newRemediation.Hash] = make(map[string]struct{})
		}

		// Add the new remediation
		store.Remediations = append(store.Remediations, newRemediation)

		// Update our map to include this new remediation
		pairKey := createPairKey(newRemediation.Pair)
		existingMap[newRemediation.Hash][pairKey] = struct{}{}

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
