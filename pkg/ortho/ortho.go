package ortho

import (
	"strconv"
	"strings"
	"time"
)

type Ortho struct {
	Grid     []interface{} `json:"grid"`
	Shape    []int         `json:"shape"`
	Position []int         `json:"position"`
	Shell    int           `json:"shell"`
	ID       string        `json:"id"`
}

func NewOrtho() *Ortho {
	return &Ortho{
		Grid:     []interface{}{},
		Shape:    []int{1},
		Position: []int{0},
		Shell:    0,
		ID:       strconv.FormatInt(time.Now().UnixNano(), 10),
	}
}

// CoordToKey serializes a slice of ints to a string key (e.g., "1,2,3")
func CoordToKey(coord []int) string {
	parts := make([]string, len(coord))
	for i, v := range coord {
		parts[i] = strconv.Itoa(v)
	}
	return strings.Join(parts, ",")
}

// KeyToCoord parses a string key back to a slice of ints
func KeyToCoord(key string) ([]int, error) {
	if key == "" {
		return []int{}, nil
	}
	parts := strings.Split(key, ",")
	coord := make([]int, len(parts))
	for i, p := range parts {
		v, err := strconv.Atoi(p)
		if err != nil {
			return nil, err
		}
		coord[i] = v
	}
	return coord, nil
}
