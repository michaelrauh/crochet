package main

import (
	"context"
	"crochet/types"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock repository service
type MockRepositoryService struct {
	mock.Mock
}

func (m *MockRepositoryService) GetContext(ctx context.Context) (types.ContextDataResponse, error) {
	args := m.Called(ctx)
	return args.Get(0).(types.ContextDataResponse), args.Error(1)
}

func (m *MockRepositoryService) GetWork(ctx context.Context) (types.WorkResponse, error) {
	args := m.Called(ctx)
	return args.Get(0).(types.WorkResponse), args.Error(1)
}

func (m *MockRepositoryService) PostResults(ctx context.Context, orthos []types.Ortho, remediations []types.RemediationTuple, receipt string) (*types.ResultsResponse, error) {
	args := m.Called(ctx, orthos, remediations, receipt)
	return args.Get(0).(*types.ResultsResponse), args.Error(1)
}

func TestProcessWorkItemWithSeedOrtho(t *testing.T) {
	// Initialize worker state with a fixed vocabulary
	voc := []string{"apple", "banana", "cherry"}
	searchState = &State{
		Version:    1,
		Vocabulary: voc,
		Pairs:      make(map[string]struct{}),
	}

	// Create a mock repository service
	mockRepo := new(MockRepositoryService)

	// Create a seed ortho (empty ortho with default shape and position)
	seedOrtho := types.Ortho{
		ID:       "seed123",
		Grid:     make(map[string]string),
		Shape:    []int{2, 2},
		Position: []int{0, 0},
		Shell:    0,
	}

	// Setup mock expectations
	// Setup PostResults to capture the orthos argument
	mockRepo.On("PostResults", mock.Anything, mock.MatchedBy(func(orthos []types.Ortho) bool {
		return len(orthos) > 0 // Just check that we're passing some orthos
	}), mock.Anything, "receipt123").
		Return(&types.ResultsResponse{
			Status:            "success",
			NewOrthosCount:    3, // Expecting one ortho for each vocabulary word
			RemediationsCount: 0,
			Version:           1, // Same version, no update needed
		}, nil)

	// Also setup GetContext in case it gets called during version check
	mockRepo.On("GetContext", mock.Anything).
		Return(types.ContextDataResponse{
			Version:    1,
			Vocabulary: voc,
			Lines:      [][]string{},
		}, nil)

	// Execute the function we want to test
	ProcessWorkItem(
		context.Background(),
		mockRepo,
		seedOrtho,
		"receipt123",
	)

	// Add a small delay to make sure all goroutines complete
	time.Sleep(50 * time.Millisecond)

	// Verify that PostResults was called with the right parameters
	mockRepo.AssertCalled(t, "PostResults", mock.Anything, mock.Anything, mock.Anything, "receipt123")

	// Get the actual orthos that were passed to PostResults
	callArgs := mockRepo.Calls[0].Arguments
	actualOrthos := callArgs.Get(1).([]types.Ortho)

	// Assertions
	assert.Len(t, actualOrthos, 3, "Should create 3 orthos (one for each vocabulary word)")

	// Verify that each ortho has one of our vocabulary words at position [0,0]
	vocabularyFound := map[string]bool{
		"apple":  false,
		"banana": false,
		"cherry": false,
	}

	for _, ortho := range actualOrthos {
		// Check if position [0,0] has a vocabulary word
		value, exists := ortho.Grid["0,0"]
		assert.True(t, exists, "Should have a value at position [0,0]")

		// Mark this vocabulary word as found
		_, isVocabWord := vocabularyFound[value]
		assert.True(t, isVocabWord, "Value should be from vocabulary")
		vocabularyFound[value] = true

		// Check other properties
		assert.Equal(t, []int{2, 2}, ortho.Shape, "Shape should be maintained")
		assert.Equal(t, 1, ortho.Shell, "Shell should be 1") // Update to correct value
		assert.NotEqual(t, "seed123", ortho.ID, "New ortho should have a different ID")
	}

	// Verify that all vocabulary words were used
	for word, found := range vocabularyFound {
		assert.True(t, found, "Vocabulary word %s should be used", word)
	}
}
