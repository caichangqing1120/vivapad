package byterange

import "testing"

func TestFindForwardBackwardAndWrap(t *testing.T) {
	value := "A\x00B\nC"
	for _, scenario := range []struct {
		caret          int
		start, end     int
		backward, wrap bool
		want           int
		found          bool
	}{
		{0, 65, 65, false, false, 0, true},
		{1, 65, 65, false, false, 0, false},
		{1, 65, 65, false, true, 0, true},
		{2, 0, 0, true, false, 1, true},
		{1, 67, 67, true, false, 0, false},
		{1, 67, 67, true, true, 4, true},
		{len(value), 67, 67, true, false, 4, true},
	} {
		got, found, err := Find(value, scenario.caret, scenario.start, scenario.end, scenario.backward, scenario.wrap)
		if err != nil || got != scenario.want || found != scenario.found {
			t.Fatalf("Find(%q, %d, %d, %d, %t, %t) = %d, %t, %v", value, scenario.caret, scenario.start, scenario.end, scenario.backward, scenario.wrap, got, found, err)
		}
	}
}

func TestFindScansUTF8BytesRatherThanCodePoints(t *testing.T) {
	value := "A猫B"
	for _, scenario := range []struct {
		caret, start, end, want int
	}{
		{0, 128, 255, 1},
		{2, 128, 255, 2},
		{3, 128, 255, 3},
		{len(value), 128, 255, 3},
	} {
		got, found, err := Find(value, scenario.caret, scenario.start, scenario.end, false, false)
		if scenario.caret == len(value) {
			got, found, err = Find(value, scenario.caret, scenario.start, scenario.end, true, false)
		}
		if err != nil || !found || got != scenario.want {
			t.Fatalf("UTF-8 byte at %d = %d, %t, %v; want %d", scenario.caret, got, found, err, scenario.want)
		}
	}
}

func TestFindRejectsInvalidRangesAndHandlesEmpty(t *testing.T) {
	for _, bounds := range [][2]int{{-1, 8}, {0, 256}, {200, 100}} {
		if _, _, err := Find("A", 0, bounds[0], bounds[1], false, false); err == nil {
			t.Fatalf("accepted range %v", bounds)
		}
	}
	if _, _, err := Find("A", 2, 0, 255, false, false); err == nil {
		t.Fatal("accepted out-of-range caret")
	}
	if _, found, err := Find("", 0, 0, 255, true, true); err != nil || found {
		t.Fatalf("empty search = %t, %v", found, err)
	}
}
