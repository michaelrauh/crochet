package main

import (
	"crochet/types"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// calculateID generates a unique identifier for a given grid and shape.
func calculateID(grid map[string]string, shape []int) string {
	// Step 1: Sort the shape
	sortedShape := append([]int(nil), shape...)
	sort.Ints(sortedShape)

	// Step 2: Extract and sort one-hot positions
	oneHotPositions := ExtractOneHotPositions(grid)
	sort.Slice(oneHotPositions, func(i, j int) bool {
		return grid[oneHotPositions[i]] < grid[oneHotPositions[j]]
	})

	// Step 3: Determine axis order
	axisOrder := DetermineAxisOrder(oneHotPositions)

	// Step 4: Sort grid entries by axis order
	gridEntries := SortGridByAxisOrder(grid, axisOrder)

	// Extract sorted positions
	sortedPositions := ExtractSortedPositions(gridEntries)

	// Step 5: Generate a unique identifier
	data := append(SerializeInts(sortedShape), SerializeStrings(sortedPositions)...)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// ExtractOneHotPositions filters and returns one-hot positions from the grid.
func ExtractOneHotPositions(grid map[string]string) []string {
	var oneHotPositions []string
	for coords := range grid {
		if SumSlice(ParseKey(coords)) == 1 {
			oneHotPositions = append(oneHotPositions, coords)
		}
	}
	return oneHotPositions
}

// DetermineAxisOrder determines the axis order based on one-hot positions.
func DetermineAxisOrder(oneHotPositions []string) []int {
	axisOrder := make([]int, len(oneHotPositions))
	for i, coords := range oneHotPositions {
		for axis, value := range ParseKey(coords) {
			if value == 1 {
				axisOrder[i] = axis
				break
			}
		}
	}
	return axisOrder
}

// SortGridByAxisOrder sorts grid entries based on the axis order.
func SortGridByAxisOrder(grid map[string]string, axisOrder []int) []gridEntry {
	gridEntries := make([]gridEntry, 0, len(grid))
	for coords, value := range grid {
		gridEntries = append(gridEntries, gridEntry{Coords: coords, Value: value})
	}
	sort.Slice(gridEntries, func(i, j int) bool {
		for _, axis := range axisOrder {
			if ParseKey(gridEntries[i].Coords)[axis] != ParseKey(gridEntries[j].Coords)[axis] {
				return ParseKey(gridEntries[i].Coords)[axis] < ParseKey(gridEntries[j].Coords)[axis]
			}
		}
		return false
	})
	return gridEntries
}

// ExtractSortedPositions extracts the sorted values from grid entries.
func ExtractSortedPositions(gridEntries []gridEntry) []string {
	sortedPositions := make([]string, len(gridEntries))
	for i, entry := range gridEntries {
		sortedPositions[i] = entry.Value
	}
	return sortedPositions
}

// SerializeInts converts a slice of integers into a byte slice.
func SerializeInts(data []int) []byte {
	result := make([]byte, len(data)*4)
	for i, v := range data {
		result[i*4] = byte(v >> 24)
		result[i*4+1] = byte(v >> 16)
		result[i*4+2] = byte(v >> 8)
		result[i*4+3] = byte(v)
	}
	return result
}

// SerializeStrings converts a slice of strings into a byte slice.
func SerializeStrings(data []string) []byte {
	var result []byte
	for _, v := range data {
		result = append(result, []byte(v)...)
	}
	return result
}

// GetOthersInSameShell identifies and collects all values in the grid that belong to the specified shell.
func GetOthersInSameShell(grid map[string]string, shell int) []string {
	uniqueValues := map[string]struct{}{}
	for pos, val := range grid {
		if SumSlice(ParseKey(pos)) == shell {
			uniqueValues[val] = struct{}{}
		}
	}
	result := make([]string, 0, len(uniqueValues))
	for val := range uniqueValues {
		result = append(result, val)
	}
	return result
}

// AllPositionsToEdge generates all positions leading to the edge of the grid along each axis.
func AllPositionsToEdge(position []int) [][][]int {
	result := make([][][]int, len(position))
	for idx, val := range position {
		axisPositions := make([][]int, val)
		for i := 0; i < val; i++ {
			newPosition := append([]int(nil), position...)
			newPosition[idx] = i
			axisPositions[i] = newPosition
		}
		result[idx] = axisPositions
	}
	return result
}

// FindAllPairPrefixes finds all non-empty prefixes of values along paths leading to the edges of the grid.
func FindAllPairPrefixes(grid map[string]string, nextPosition []int) [][]string {
	// Generate all positions leading to the edge for each axis
	edgePositions := AllPositionsToEdge(nextPosition)

	// Map over each dimension's positions
	var result [][]string
	for _, dimensionPositions := range edgePositions {
		var dimensionValues []string
		for _, pos := range dimensionPositions {
			if val, exists := grid[FormatKey(pos)]; exists {
				dimensionValues = append(dimensionValues, val)
			}
		}
		if len(dimensionValues) > 0 {
			result = append(result, dimensionValues)
		}
	}

	return result
}

// PadGrid prepends a 0 to the keys of the grid and returns a new grid.
func PadGrid(grid map[string]string) map[string]string {
	paddedGrid := make(map[string]string)
	for key, value := range grid {
		newKey := "0," + key // Prepend 0 to the key
		paddedGrid[newKey] = value
	}
	return paddedGrid
}

// CopyGridWithItem creates a copy of the grid and adds the item at the specified position.
func CopyGridWithItem(grid map[string]string, position []int, item string) map[string]string {
	newGrid := make(map[string]string)
	for k, v := range grid {
		newGrid[k] = v
	}
	// Ensure the position is padded to match the grid's dimensionality
	paddedPosition := PadPosition(position, len(ParseKey(GridKeyExample(grid))))
	newGrid[FormatKey(paddedPosition)] = item
	return newGrid
}

// PadPosition ensures the position slice matches the required length by appending zeros if necessary.
func PadPosition(position []int, requiredLength int) []int {
	if len(position) >= requiredLength {
		return position
	}
	padding := make([]int, requiredLength-len(position))
	return append(position, padding...)
}

// GridKeyExample retrieves an example key from the grid to determine its dimensionality.
func GridKeyExample(grid map[string]string) string {
	for key := range grid {
		return key
	}
	return ""
}

// PrependZero prepends a 0 to the position for handling "both" cases.
func PrependZero(position []int) []int {
	return append([]int{0}, position...)
}

// FormatKey converts a slice of integers into a string key.
func FormatKey(position []int) string {
	return strings.Trim(strings.Join(strings.Fields(fmt.Sprint(position)), ","), "[]")
}

// ParseKey converts a string key back into a slice of integers.
func ParseKey(key string) []int {
	parts := strings.Split(key, ",")
	result := make([]int, len(parts))
	for i, part := range parts {
		result[i], _ = strconv.Atoi(part)
	}
	return result
}

// SumSlice calculates the sum of elements in a slice.
func SumSlice(slice []int) int {
	total := 0
	for _, v := range slice {
		total += v
	}
	return total
}

// gridEntry represents a grid coordinate and its associated value.
type gridEntry struct {
	Coords string
	Value  string
}

// GetRequirements calculates forbidden and required values for the given Ortho structure.
func GetRequirements(ortho types.Ortho) (forbidden []string, required [][]string) {
	// Get forbidden values from the same shell
	forbidden = GetOthersInSameShell(ortho.Grid, ortho.Shell)
	// Get required prefixes along paths to the edges
	required = FindAllPairPrefixes(ortho.Grid, ortho.Position)
	return forbidden, required
}

// Add updates the Ortho struct by adding an item to the grid and handling transitions.
func Add(ortho types.Ortho, item string, counter *Counter) []types.Ortho {
	kind, results := counter.Increment(ortho.Shape, ortho.Position)

	switch kind {
	case "same":
		// Handle the "same" case
		newShape := results[0].NewShape
		nextPosition := results[0].NewPos
		shell := results[0].Sum

		newGrid := CopyGridWithItem(ortho.Grid, ortho.Position, item)
		newID := calculateID(newGrid, newShape)

		return []types.Ortho{
			{
				Grid:     newGrid,
				Position: nextPosition,
				Shape:    newShape,
				Shell:    shell,
				ID:       newID,
			},
		}

	case "both":
		// Handle the "both" case
		upShape := results[0].NewShape
		upPosition := results[0].NewPos
		upShell := results[0].Sum

		upGrid := PadGrid(ortho.Grid)
		newUpGrid := CopyGridWithItem(upGrid, PrependZero(ortho.Position), item)
		newUpID := calculateID(newUpGrid, upShape)

		overOrthos := []types.Ortho{
			{
				Grid:     newUpGrid,
				Position: upPosition,
				Shape:    upShape,
				Shell:    upShell,
				ID:       newUpID,
			},
		}

		for _, result := range results[1:] {
			newShape := result.NewShape
			nextPosition := result.NewPos
			shell := result.Sum

			newGrid := CopyGridWithItem(ortho.Grid, ortho.Position, item)
			newID := calculateID(newGrid, newShape)

			overOrthos = append(overOrthos, types.Ortho{
				Grid:     newGrid,
				Position: nextPosition,
				Shape:    newShape,
				Shell:    shell,
				ID:       newID,
			})
		}

		return overOrthos

	case "over":
		// Handle the "over" case
		var overOrthos []types.Ortho
		for _, result := range results {
			newShape := result.NewShape
			nextPosition := result.NewPos
			shell := result.Sum

			newGrid := CopyGridWithItem(ortho.Grid, ortho.Position, item)
			newID := calculateID(newGrid, newShape)

			overOrthos = append(overOrthos, types.Ortho{
				Grid:     newGrid,
				Position: nextPosition,
				Shape:    newShape,
				Shell:    shell,
				ID:       newID,
			})
		}

		return overOrthos

	default:
		return nil
	}
}

// newOrtho creates and returns a new Ortho struct with default values.
func newOrtho() types.Ortho {
	return types.Ortho{
		Grid:     make(map[string]string), // Empty grid
		Shape:    []int{2, 2},             // Default shape
		Position: []int{0, 0},             // Default starting position
		Shell:    0,                       // Default shell
		ID:       "",                      // Default ID (empty string)
	}
}
