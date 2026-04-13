package tui

import "testing"

func TestShouldStack(t *testing.T) {
	if shouldStack(120, 40) {
		t.Fatal("did not expect wide terminal to stack")
	}
	if !shouldStack(99, 40) {
		t.Fatal("expected narrow terminal to stack")
	}
	if !shouldStack(120, 29) {
		t.Fatal("expected short terminal to stack")
	}
}

func TestWeightedWidths(t *testing.T) {
	left, center, right := weightedWidths(100, []int{1, 2, 2})
	if left+center+right != 100 {
		t.Fatalf("widths do not sum to total: %d %d %d", left, center, right)
	}
	if left != 20 || center != 40 || right != 40 {
		t.Fatalf("unexpected widths: %d %d %d", left, center, right)
	}
}
