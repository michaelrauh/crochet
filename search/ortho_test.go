package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOrthoRequirements(t *testing.T) {
	ortho := newOrtho()

	// Test querying requirements for a new Ortho
	forbidden, required := GetRequirements(ortho)
	assert.Empty(t, forbidden)
	assert.Empty(t, required)
}

func TestOrthoForbiddenAndRequired(t *testing.T) {
	counter := NewCounter() // Reuse a single Counter instance
	ortho := newOrtho()

	// Add items to the Ortho
	orthos := Add(ortho, "a", counter)
	ortho = orthos[0]
	orthos = Add(ortho, "b", counter)
	ortho = orthos[0]

	// Test forbidden and required values
	forbidden, required := GetRequirements(ortho)
	assert.ElementsMatch(t, forbidden, []string{"b"})
	assert.ElementsMatch(t, required, [][]string{{"a"}})
}

func TestOrthoAddAgain(t *testing.T) {
	counter := NewCounter() // Reuse a single Counter instance
	ortho := newOrtho()

	// Add multiple items to the Ortho
	orthos := Add(ortho, "a", counter)
	ortho = orthos[0]
	orthos = Add(ortho, "b", counter)
	ortho = orthos[0]
	orthos = Add(ortho, "c", counter)
	ortho = orthos[0]
	orthos = Add(ortho, "d", counter)
	ortho = orthos[0]
	_ = Add(ortho, "e", counter)
}

func TestOrthoSharedIDs(t *testing.T) {
	counter := NewCounter() // Reuse a single Counter instance
	ortho1 := newOrtho()
	ortho2 := newOrtho()

	// Add items to both orthos in different orders
	orthos := Add(ortho1, "a", counter)
	ortho1 = orthos[0]
	orthos = Add(ortho2, "a", counter)
	ortho2 = orthos[0]
	assert.Equal(t, ortho1.ID, ortho2.ID)

	orthos = Add(ortho1, "b", counter)
	ortho1 = orthos[0]
	orthos = Add(ortho2, "c", counter)
	ortho2 = orthos[0]
	assert.NotEqual(t, ortho1.ID, ortho2.ID)

	orthos = Add(ortho1, "c", counter)
	ortho1 = orthos[0]
	orthos = Add(ortho2, "b", counter)
	ortho2 = orthos[0]
	assert.Equal(t, ortho1.ID, ortho2.ID)

	orthos = Add(ortho1, "d", counter)
	ortho1 = orthos[0]
	orthos = Add(ortho2, "d", counter)
	ortho2 = orthos[0]
	assert.Equal(t, ortho1.ID, ortho2.ID)
}

func TestOrthoBuildOver(t *testing.T) {
	counter := NewCounter() // Reuse a single Counter instance
	ortho := newOrtho()

	// Add items to the Ortho and test shape evolution
	orthos := Add(ortho, "a", counter)
	ortho = orthos[0]
	orthos = Add(ortho, "b", counter)
	ortho = orthos[0]
	orthos = Add(ortho, "c", counter)
	ortho = orthos[0]
	orthos = Add(ortho, "d", counter)
	ortho = orthos[1]
	orthos = Add(ortho, "e", counter)
	ortho = orthos[0]
	assert.Equal(t, []int{3, 2}, ortho.Shape)

	orthos = Add(ortho, "f", counter)
	ortho = orthos[1]
	orthos = Add(ortho, "g", counter)
	ortho = orthos[0]
	assert.Equal(t, []int{3, 3}, ortho.Shape)
}
