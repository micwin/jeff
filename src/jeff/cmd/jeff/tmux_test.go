package main

import "testing"

func TestParseTriple(t *testing.T) {
	l, c, r, err := parseTriple("2:4:2")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if l != 2 || c != 4 || r != 2 {
		t.Fatalf("unexpected values: %d %d %d", l, c, r)
	}

	if _, _, _, err := parseTriple("2:0:1"); err == nil {
		t.Fatalf("expected error for zero value")
	}
	if _, _, _, err := parseTriple("bad"); err == nil {
		t.Fatalf("expected error for malformed input")
	}
}
