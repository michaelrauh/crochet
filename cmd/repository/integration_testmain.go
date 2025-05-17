//go:build integration
// +build integration

package main

import (
	"fmt"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	fmt.Println("In testmain")
	os.Exit(m.Run())
}
