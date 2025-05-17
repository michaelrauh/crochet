package ortho

import (
	"encoding/json"
	"testing"
)

func TestNewOrtho(t *testing.T) {
	o := NewOrtho()

	if o == nil {
		t.Fatal("NewOrtho returned nil")
	}
	if len(o.Grid) != 0 {
		t.Errorf("expected empty Grid, got %v", o.Grid)
	}
	if len(o.Shape) != 1 {
		t.Errorf("expected empty Shape, got %v", o.Shape)
	}
	if o.Shape[0] != 1 {
		t.Errorf("expected Shape[0] 1, got %v", o.Shape[0])
	}
	if len(o.Position) != 1 {
		t.Errorf("expected empty Position, got %v", o.Position)
	}
	if o.Position[0] != 0 {
		t.Errorf("expected Position[0] 0, got %v", o.Position[0])
	}
	if o.Shell != 0 {
		t.Errorf("expected Shell 0, got %v", o.Shell)
	}
	if o.ID == "" {
		t.Errorf("expected nonempty ID, got %v", o.ID)
	}
}

func TestOrthoJSONSerialization(t *testing.T) {
	o := NewOrtho()
	_, err := json.Marshal(o)
	if err != nil {
		t.Errorf("Ortho should be serializable to JSON, got error: %v", err)
	}
}
