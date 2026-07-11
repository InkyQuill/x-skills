package ui

import (
	"reflect"
	"testing"
)

func TestClampIndex(t *testing.T) {
	tests := []struct {
		name         string
		index, count int
		want         int
	}{
		{name: "empty", index: 4, count: 0, want: 0},
		{name: "negative count", index: 4, count: -1, want: 0},
		{name: "negative index", index: -1, count: 3, want: 0},
		{name: "in range", index: 1, count: 3, want: 1},
		{name: "past end", index: 3, count: 3, want: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClampIndex(tt.index, tt.count); got != tt.want {
				t.Fatalf("ClampIndex(%d, %d) = %d, want %d", tt.index, tt.count, got, tt.want)
			}
		})
	}
}

func TestClampScroll(t *testing.T) {
	tests := []struct {
		name                               string
		scroll, bodyHeight, viewportHeight int
		want                               int
	}{
		{name: "negative scroll", scroll: -1, bodyHeight: 10, viewportHeight: 3, want: 0},
		{name: "empty body", scroll: 5, bodyHeight: 0, viewportHeight: 3, want: 0},
		{name: "negative body", scroll: 5, bodyHeight: -1, viewportHeight: 3, want: 0},
		{name: "non-positive viewport", scroll: 5, bodyHeight: 10, viewportHeight: 0, want: 0},
		{name: "exact fit", scroll: 5, bodyHeight: 3, viewportHeight: 3, want: 0},
		{name: "in range", scroll: 4, bodyHeight: 10, viewportHeight: 3, want: 4},
		{name: "past end", scroll: 1000, bodyHeight: 10, viewportHeight: 3, want: 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClampScroll(tt.scroll, tt.bodyHeight, tt.viewportHeight)
			if got != tt.want {
				t.Fatalf("ClampScroll(%d, %d, %d) = %d, want %d", tt.scroll, tt.bodyHeight, tt.viewportHeight, got, tt.want)
			}
		})
	}
}

func TestVisibleLines(t *testing.T) {
	lines := []string{"zero", "one", "two", "three"}
	original := append([]string(nil), lines...)

	tests := []struct {
		name           string
		scroll, height int
		want           []string
	}{
		{name: "negative height", scroll: 0, height: -1, want: []string{}},
		{name: "zero height", scroll: 0, height: 0, want: []string{}},
		{name: "negative scroll", scroll: -2, height: 2, want: []string{"zero", "one"}},
		{name: "exact fit", scroll: 0, height: 4, want: lines},
		{name: "excessive scroll", scroll: 1000, height: 2, want: []string{"two", "three"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VisibleLines(lines, tt.scroll, tt.height)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("VisibleLines() = %#v, want %#v", got, tt.want)
			}
			if len(got) > 0 {
				got[0] = "changed"
			}
			if !reflect.DeepEqual(lines, original) {
				t.Fatalf("VisibleLines() mutated input: %#v", lines)
			}
		})
	}
}
