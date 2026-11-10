package main

import "testing"

func TestRectanglePositionMappingUsesRunesAcrossLines(t *testing.T) {
	before := []string{"猫a", "界b"}
	after := []string{"猫中a", "界中b"}
	for _, test := range []struct{ old, want int }{{1, 2}, {4, 6}} {
		if got, ok := mapRectanglePosition(before, after, test.old); !ok || got != test.want {
			t.Fatalf("mapped rune %d to %d, want %d; ok=%v", test.old, got, test.want, ok)
		}
	}
	before = []string{"aXYb", "cXYd"}
	after = []string{"aXb", "cXd"}
	for _, test := range []struct{ old, want int }{{3, 2}, {8, 6}} {
		if got, ok := mapRectanglePosition(before, after, test.old); !ok || got != test.want {
			t.Fatalf("mapped deletion %d to %d, want %d; ok=%v", test.old, got, test.want, ok)
		}
	}
}
