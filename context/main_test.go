package main

import (
	"testing"
)

func TestMain(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("main function panicked: %v", r)
		}
	}()

	main()
}
