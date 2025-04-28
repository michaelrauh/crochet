package main

import (
	"sort"
	"strings"
)

// Counter represents a simple counter structure
type Counter struct{}

// NewCounter creates and returns a new Counter
func NewCounter() *Counter {
	return &Counter{}
}

// ShapePosition represents a tuple of a shape and a position.
type ShapePosition struct {
	Shape    []int
	Position []int
}

// GetNext determines the next valid position based on the current shape and position.
func (c *Counter) GetNext(shape, currentPosition []int) (string, []ShapePosition) {

	visited := GetVisited(shape, currentPosition)

	possibilities := SortedCartesian(shape)

	// Filter out visited possibilities
	validPossibilities := [][]int{}
	for _, pos := range possibilities {
		if !contains(visited, pos) {
			validPossibilities = append(validPossibilities, pos)
		}
	}

	// Find the index of the current position in valid possibilities
	currentIndex := -1
	for i, pos := range validPossibilities {
		if slicesEqual(pos, currentPosition) {
			currentIndex = i
			break
		}
	}

	// Determine the next position
	if currentIndex+1 < len(validPossibilities) {
		// "same" case: move to the next valid position
		desired := validPossibilities[currentIndex+1]

		InsertVisited(shape, desired, append(visited, currentPosition))
		return "same", []ShapePosition{
			{Shape: shape, Position: desired},
		}
	}

	// If no next position, handle "over" or "both" cases
	overList := OverShapes(shape, possibilities)

	if allEqual(shape, 2) {
		// "both" case: extend the shape and handle over shapes
		newShape, newPos := UpShape(shape, possibilities)

		// Update VisitedMap for the new shape and position
		InsertVisited(newShape, newPos, append(visited, append([]int{0}, currentPosition...)))
		for _, item := range overList {
			InsertVisited(item.NewShape, item.NewPos, append(visited, currentPosition))
		}

		results := []ShapePosition{
			{Shape: newShape, Position: newPos},
		}
		for _, item := range overList {
			results = append(results, ShapePosition{Shape: item.NewShape, Position: item.NewPos})
		}
		return "both", results
	}

	// "over" case: handle over shapes
	results := []ShapePosition{}
	for _, item := range overList {
		InsertVisited(item.NewShape, item.NewPos, append(visited, currentPosition))
		results = append(results, ShapePosition{Shape: item.NewShape, Position: item.NewPos})
	}
	return "over", results
}

// Increment determines the next state for a given shape and current position, caching results in a map.
func (c *Counter) Increment(shape, currentPosition []int) (string, []struct {
	NewShape []int
	NewPos   []int
	Sum      int
}) {
	key := generateKey(shape, currentPosition)

	// Check if the result is already cached
	if cached, exists := VisitedMap[key]; exists {
		if cachedResult, ok := cached.(struct {
			Type    string
			Results []struct {
				NewShape []int
				NewPos   []int
				Sum      int
			}
		}); ok {
			return cachedResult.Type, cachedResult.Results
		}
	}

	// Get the next state
	stateType, shapePositions := c.GetNext(shape, currentPosition)

	// Add the sum of each position to the results
	resultsWithSums := []struct {
		NewShape []int
		NewPos   []int
		Sum      int
	}{}
	for _, sp := range shapePositions {
		resultsWithSums = append(resultsWithSums, struct {
			NewShape []int
			NewPos   []int
			Sum      int
		}{
			NewShape: sp.Shape,
			NewPos:   sp.Position,
			Sum:      sum(sp.Position),
		})
	}

	// Cache the result as a struct with Type and Results
	VisitedMap[key] = struct {
		Type    string
		Results []struct {
			NewShape []int
			NewPos   []int
			Sum      int
		}
	}{
		Type:    stateType,
		Results: resultsWithSums,
	}

	return stateType, resultsWithSums
}

// contains checks if a slice of slices contains a specific slice.
func contains(slices [][]int, target []int) bool {
	for _, slice := range slices {
		if slicesEqual(slice, target) {
			return true
		}
	}
	return false
}

// slicesEqual checks if two slices are equal.
func slicesEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// allEqual checks if all elements in a slice are equal to a given value.
func allEqual(slice []int, value int) bool {
	for _, v := range slice {
		if v != value {
			return false
		}
	}
	return true
}

// VisitedMap is a global map to store visited states.
var VisitedMap = make(map[string]interface{})

// GetVisited retrieves the previously stored value for a given shape and position.
// If no value exists, it returns an empty slice of slices.
func GetVisited(shape, position []int) [][]int {
	key := generateKey(shape, position)
	if value, exists := VisitedMap[key]; exists {
		if visited, ok := value.([][]int); ok {
			return visited
		}
	}
	return [][]int{} // Return an empty slice if no value exists or type assertion fails
}

// InsertVisited stores a value for a given shape and position.
// Ensure the value is always of type [][]int and properly appends new visited positions.
func InsertVisited(shape, position []int, previous [][]int) {
	key := generateKey(shape, position)
	if existing, exists := VisitedMap[key]; exists {
		if visited, ok := existing.([][]int); ok {
			// Avoid duplicate entries in visited positions
			updatedVisited := append(visited, previous...)
			VisitedMap[key] = removeDuplicates(updatedVisited)
			return
		}
	}
	VisitedMap[key] = removeDuplicates(previous)
}

// removeDuplicates removes duplicate slices from a slice of slices.
func removeDuplicates(slices [][]int) [][]int {
	unique := make(map[string]struct{})
	var result [][]int
	for _, slice := range slices {
		key := sliceToKey(slice)
		if _, exists := unique[key]; !exists {
			unique[key] = struct{}{}
			result = append(result, slice)
		}
	}
	return result
}

// generateKey creates a unique key for a combination of shape and position.
func generateKey(shape, position []int) string {
	var sb strings.Builder
	for _, v := range append(shape, position...) {
		sb.WriteString(",")
		sb.WriteString(string(rune(v)))
	}
	return sb.String()
}

// CartesianProduct generates the Cartesian product of a slice of integers.
func CartesianProduct(shape []int) [][]int {
	if len(shape) == 0 {
		return [][]int{{}}
	}

	rest := CartesianProduct(shape[1:])
	result := make([][]int, 0, shape[0]*len(rest))
	for i := 0; i < shape[0]; i++ { // Iterate up to the value of the first dimension
		for _, r := range rest {
			result = append(result, append([]int{i}, r...))
		}
	}
	return result
}

// SortedCartesian generates the Cartesian product of a shape and sorts it by sum and lexicographical order.
func SortedCartesian(shape []int) [][]int {
	product := CartesianProduct(shape)

	sort.Slice(product, func(i, j int) bool {
		sumI, sumJ := sum(product[i]), sum(product[j])
		if sumI == sumJ {
			return lexicographicalLess(product[i], product[j])
		}
		return sumI < sumJ
	})

	return product
}

// sum calculates the sum of elements in a slice.
func sum(slice []int) int {
	total := 0
	for _, v := range slice {
		total += v
	}
	return total
}

// lexicographicalLess compares two slices lexicographically.
func lexicographicalLess(a, b []int) bool {
	for i := range a {
		if i >= len(b) || a[i] != b[i] {
			return i < len(b) && a[i] < b[i]
		}
	}
	return len(a) < len(b)
}

// DistinctLocations returns the indices of the first occurrence of each distinct value in the input slice.
func DistinctLocations(shape []int) []int {
	seen := make(map[int]struct{})
	var indices []int

	for i, val := range shape {
		if _, exists := seen[val]; !exists {
			seen[val] = struct{}{}
			indices = append(indices, i)
		}
	}

	return indices
}

// UpShape takes a shape and existing positions, and returns a new shape and the first valid position.
func UpShape(shape []int, existing [][]int) ([]int, []int) {
	// Extend the shape by prepending 2
	newShape := append([]int{2}, shape...)

	// Generate the Cartesian product of the new shape
	cartesian := SortedCartesian(newShape)

	// Transform existing positions by prepending 0 to each
	transformedExisting := make(map[string]struct{}, len(existing))
	for _, pos := range existing {
		transformed := append([]int{0}, pos...)
		transformedExisting[sliceToKey(transformed)] = struct{}{}
	}

	// Find the first position not in transformedExisting
	for _, pos := range cartesian {
		if _, exists := transformedExisting[sliceToKey(pos)]; !exists {
			return newShape, pos
		}
	}

	// Return empty position if no valid position is found
	return newShape, nil
}

// sliceToKey converts a slice of integers to a string key for use in a map.
func sliceToKey(slice []int) string {
	var sb strings.Builder
	for i, v := range slice {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(string(rune(v)))
	}
	return sb.String()
}

// OverShapes iterates over distinct locations in the shape, updates the shape, and finds the first valid position.
func OverShapes(shape []int, existing [][]int) []struct {
	NewShape []int
	NewPos   []int
} {
	results := []struct {
		NewShape []int
		NewPos   []int
	}{}

	// Create a set of existing positions for quick lookup
	existingSet := make(map[string]struct{}, len(existing))
	for _, pos := range existing {
		existingSet[sliceToKey(pos)] = struct{}{}
	}

	// Iterate over distinct locations in the shape
	for _, location := range DistinctLocations(shape) {
		// Create a new shape by incrementing the value at the current location
		newShape := append([]int(nil), shape...) // Copy the shape
		newShape[location]++

		// Generate the Cartesian product of the new shape
		cartesian := SortedCartesian(newShape)

		// Find the first position not in existingSet
		var newPos []int
		for _, pos := range cartesian {
			if _, exists := existingSet[sliceToKey(pos)]; !exists {
				newPos = pos
				break
			}
		}

		// Append the result as a struct with NewShape and NewPos
		results = append(results, struct {
			NewShape []int
			NewPos   []int
		}{
			NewShape: newShape,
			NewPos:   newPos,
		})
	}

	return results
}
