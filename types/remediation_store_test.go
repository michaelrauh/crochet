// filepath: /Users/michaelrauh/dev/crochet/types/remediation_store_test.go
package types

import (
	"testing"
)

func TestSaveRemediationsToStore(t *testing.T) {
	store := InitRemediationStore()

	// Test 1: Add a single new remediation
	remediation1 := RemediationTuple{
		Pair: []string{"word1", "word2"},
		Hash: "hash1",
	}

	count := SaveRemediationsToStore(store, []RemediationTuple{remediation1})
	if count != 1 {
		t.Errorf("Expected to add 1 remediation, but added %d", count)
	}
	if len(store.Remediations) != 1 {
		t.Errorf("Expected store to have 1 remediation, but has %d", len(store.Remediations))
	}

	// Test 2: Try to add the same remediation again (should not be added)
	count = SaveRemediationsToStore(store, []RemediationTuple{remediation1})
	if count != 0 {
		t.Errorf("Expected to add 0 remediation when adding duplicate, but added %d", count)
	}
	if len(store.Remediations) != 1 {
		t.Errorf("Expected store to still have 1 remediation, but has %d", len(store.Remediations))
	}

	// Test 3: Add a remediation with the same hash but different pair
	remediation2 := RemediationTuple{
		Pair: []string{"word3", "word4"},
		Hash: "hash1",
	}

	count = SaveRemediationsToStore(store, []RemediationTuple{remediation2})
	if count != 1 {
		t.Errorf("Expected to add 1 remediation with same hash but different pair, but added %d", count)
	}
	if len(store.Remediations) != 2 {
		t.Errorf("Expected store to have 2 remediations, but has %d", len(store.Remediations))
	}

	// Test 4: Add multiple remediations at once, some new and some duplicates
	remediation3 := RemediationTuple{
		Pair: []string{"word5", "word6"},
		Hash: "hash2",
	}

	count = SaveRemediationsToStore(store, []RemediationTuple{remediation1, remediation2, remediation3})
	if count != 1 {
		t.Errorf("Expected to add 1 new remediation when adding mix of new and duplicates, but added %d", count)
	}
	if len(store.Remediations) != 3 {
		t.Errorf("Expected store to have 3 remediations, but has %d", len(store.Remediations))
	}

	// Test 5: Performance test with larger dataset
	// Create a store with many existing remediations
	largeStore := InitRemediationStore()

	// Add 100 initial remediations
	initialRemediations := make([]RemediationTuple, 100)
	for i := 0; i < 100; i++ {
		initialRemediations[i] = RemediationTuple{
			Pair: []string{"initial", "word" + string(rune('a'+i))},
			Hash: "hash" + string(rune('a'+i)),
		}
	}

	SaveRemediationsToStore(largeStore, initialRemediations)

	// Try to add 50 more remediations, 25 new and 25 duplicates
	newRemediations := make([]RemediationTuple, 50)
	for i := 0; i < 25; i++ {
		// Duplicate existing remediations
		newRemediations[i] = initialRemediations[i]
	}
	for i := 25; i < 50; i++ {
		// New remediations
		newRemediations[i] = RemediationTuple{
			Pair: []string{"new", "word" + string(rune('a'+i))},
			Hash: "newhash" + string(rune('a'+i)),
		}
	}

	count = SaveRemediationsToStore(largeStore, newRemediations)
	if count != 25 {
		t.Errorf("Expected to add 25 new remediations in performance test, but added %d", count)
	}
	if len(largeStore.Remediations) != 125 {
		t.Errorf("Expected store to have 125 remediations after performance test, but has %d", len(largeStore.Remediations))
	}
}
