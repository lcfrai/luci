package main

import "testing"

func TestMakePDFCountsCompletedTrailerInAggregate(t *testing.T) {
	font := []byte("small-cff")
	cmap := []byte("small-cmap")
	widths := []int{100, 200, 300}
	box := [4]int{0, 0, 100, 100}
	baseline, err := makePDF(font, cmap, widths, box, 0)
	if err != nil {
		t.Fatal(err)
	}
	remaining := maxCombinedBytes - len(font) - len(baseline)
	if remaining <= 0 {
		t.Fatal("fixture has no aggregate boundary")
	}
	if _, err := makePDF(font, cmap, widths, box, remaining); err != nil {
		t.Fatalf("exact aggregate bound refused: %v", err)
	}
	if _, err := makePDF(font, cmap, widths, box, remaining+1); err == nil {
		t.Fatal("completed trailer exceeded aggregate without refusal")
	}
}
