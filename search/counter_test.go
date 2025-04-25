package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetNext(t *testing.T) {
	counter := NewCounter()

	// Test cases for [2, 2]
	kind, results := counter.GetNext([]int{2, 2}, []int{0, 0})
	assert.Equal(t, "same", kind)
	assert.Equal(t, []ShapePosition{
		{Shape: []int{2, 2}, Position: []int{0, 1}},
	}, results)

	kind, results = counter.GetNext([]int{2, 2}, []int{0, 1})
	assert.Equal(t, "same", kind)
	assert.Equal(t, []ShapePosition{
		{Shape: []int{2, 2}, Position: []int{1, 0}},
	}, results)

	kind, results = counter.GetNext([]int{2, 2}, []int{1, 0})
	assert.Equal(t, "same", kind)
	assert.Equal(t, []ShapePosition{
		{Shape: []int{2, 2}, Position: []int{1, 1}},
	}, results)

	kind, results = counter.GetNext([]int{2, 2}, []int{1, 1})
	assert.Equal(t, "both", kind)
	assert.Equal(t, []ShapePosition{
		{Shape: []int{2, 2, 2}, Position: []int{1, 0, 0}},
		{Shape: []int{3, 2}, Position: []int{2, 0}},
	}, results)

	// Test cases for [2, 2, 2]
	kind, results = counter.GetNext([]int{2, 2, 2}, []int{1, 0, 0})
	assert.Equal(t, "same", kind)
	assert.Equal(t, []ShapePosition{
		{Shape: []int{2, 2, 2}, Position: []int{1, 0, 1}},
	}, results)

	kind, results = counter.GetNext([]int{2, 2, 2}, []int{1, 0, 1})
	assert.Equal(t, "same", kind)
	assert.Equal(t, []ShapePosition{
		{Shape: []int{2, 2, 2}, Position: []int{1, 1, 0}},
	}, results)

	kind, results = counter.GetNext([]int{2, 2, 2}, []int{1, 1, 0})
	assert.Equal(t, "same", kind)
	assert.Equal(t, []ShapePosition{
		{Shape: []int{2, 2, 2}, Position: []int{1, 1, 1}},
	}, results)

	kind, results = counter.GetNext([]int{2, 2, 2}, []int{1, 1, 1})
	assert.Equal(t, "both", kind)
	assert.Equal(t, []ShapePosition{
		{Shape: []int{2, 2, 2, 2}, Position: []int{1, 0, 0, 0}},
		{Shape: []int{3, 2, 2}, Position: []int{2, 0, 0}},
	}, results)

	// Test cases for [3, 2]
	kind, results = counter.GetNext([]int{3, 2}, []int{2, 0})
	assert.Equal(t, "same", kind)
	assert.Equal(t, []ShapePosition{
		{Shape: []int{3, 2}, Position: []int{2, 1}},
	}, results)

	kind, results = counter.GetNext([]int{3, 2}, []int{2, 1})
	assert.Equal(t, "over", kind)
	assert.Equal(t, []ShapePosition{
		{Shape: []int{4, 2}, Position: []int{3, 0}},
		{Shape: []int{3, 3}, Position: []int{0, 2}},
	}, results)

	// Test cases for [4, 2]
	kind, results = counter.GetNext([]int{4, 2}, []int{3, 0})
	assert.Equal(t, "same", kind)
	assert.Equal(t, []ShapePosition{
		{Shape: []int{4, 2}, Position: []int{3, 1}},
	}, results)

	kind, results = counter.GetNext([]int{4, 2}, []int{3, 1})
	assert.Equal(t, "over", kind)
	assert.Equal(t, []ShapePosition{
		{Shape: []int{5, 2}, Position: []int{4, 0}},
		{Shape: []int{4, 3}, Position: []int{0, 2}},
	}, results)

	// Test cases for [3, 3]
	kind, results = counter.GetNext([]int{3, 3}, []int{0, 2})
	assert.Equal(t, "same", kind)
	assert.Equal(t, []ShapePosition{
		{Shape: []int{3, 3}, Position: []int{1, 2}},
	}, results)

	kind, results = counter.GetNext([]int{3, 3}, []int{1, 2})
	assert.Equal(t, "same", kind)
	assert.Equal(t, []ShapePosition{
		{Shape: []int{3, 3}, Position: []int{2, 2}},
	}, results)

	kind, results = counter.GetNext([]int{3, 3}, []int{2, 2})
	assert.Equal(t, "over", kind)
	assert.Equal(t, []ShapePosition{
		{Shape: []int{4, 3}, Position: []int{3, 0}},
	}, results)
}

func TestIncrement(t *testing.T) {
	counter := NewCounter()

	// Test cases for [2, 2]
	kind, results := counter.Increment([]int{2, 2}, []int{0, 0})
	assert.Equal(t, "same", kind)
	assert.Equal(t, []struct {
		NewShape []int
		NewPos   []int
		Sum      int
	}{
		{NewShape: []int{2, 2}, NewPos: []int{0, 1}, Sum: 1},
	}, results)

	kind, results = counter.Increment([]int{2, 2}, []int{0, 1})
	assert.Equal(t, "same", kind)
	assert.Equal(t, []struct {
		NewShape []int
		NewPos   []int
		Sum      int
	}{
		{NewShape: []int{2, 2}, NewPos: []int{1, 0}, Sum: 1},
	}, results)

	kind, results = counter.Increment([]int{2, 2}, []int{1, 0})
	assert.Equal(t, "same", kind)
	assert.Equal(t, []struct {
		NewShape []int
		NewPos   []int
		Sum      int
	}{
		{NewShape: []int{2, 2}, NewPos: []int{1, 1}, Sum: 2},
	}, results)

	// Test case for "both" transition
	kind, results = counter.Increment([]int{2, 2}, []int{1, 1})
	assert.Equal(t, "both", kind)
	assert.Equal(t, []struct {
		NewShape []int
		NewPos   []int
		Sum      int
	}{
		{NewShape: []int{2, 2, 2}, NewPos: []int{1, 0, 0}, Sum: 1},
		{NewShape: []int{3, 2}, NewPos: []int{2, 0}, Sum: 2},
	}, results)
}
