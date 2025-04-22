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
	addedCount := 0

	for _, newRemediation := range remediations {
		// Check if remediation already exists
		exists := false
		for _, existingRemediation := range store.Remediations {
			// Compare hash and pair
			if existingRemediation.Hash == newRemediation.Hash &&
				CompareStringSlices(existingRemediation.Pair, newRemediation.Pair) {
				exists = true
				break
			}
		}

		if !exists {
			store.Remediations = append(store.Remediations, newRemediation)
			addedCount++
		}
	}

	return addedCount
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
