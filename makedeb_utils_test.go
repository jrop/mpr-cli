package main

import (
	"testing"
)

func TestParseMakedebG(t *testing.T) {
	input := []byte(`sha256sums1=123
sha256sums2=456
sha256sums3=789`)

	vars := parseMakedebG(input)
	if len(vars) != 3 {
		t.Errorf("expected 3 vars, got %d", len(vars))
	}

	if vars["sha256sums1"] != "123" {
		t.Errorf("expected sha256sums1 to be 123, got %s", vars["sha256sums1"])
	}
	if vars["sha256sums2"] != "456" {
		t.Errorf("expected sha256sums2 to be 456, got %s", vars["sha256sums2"])
	}
	if vars["sha256sums3"] != "789" {
		t.Errorf("expected sha256sums3 to be 789, got %s", vars["sha256sums3"])
	}
}
