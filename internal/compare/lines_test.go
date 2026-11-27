package compare

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

func TestDiffLinesSeparatesChangeBlocks(t *testing.T) {
	result := DiffLines("zero\nold\nanchor\ndelete\nanchor2\nlast\n", "zero\nnew\nanchor\nanchor2\ninsert\nlast\n")
	want := []Hunk{{1, 2, 1, 2}, {3, 4, 3, 3}, {5, 5, 4, 5}}
	if !reflect.DeepEqual(result.Hunks, want) {
		t.Fatalf("hunks = %v, want %v", result.Hunks, want)
	}
	for _, item := range []struct{ side, line, want int }{{0, 4, 3}, {1, 3, 4}, {0, 5, 5}, {1, 4, 5}, {0, 6, 6}} {
		if got := result.PeerLine(item.side, item.line); got != item.want {
			t.Errorf("PeerLine(%d, %d) = %d, want %d", item.side, item.line, got, item.want)
		}
	}
}

func TestDiffLinesEmptyAndFinalNewline(t *testing.T) {
	for _, item := range []struct {
		left, right string
		want        []Hunk
	}{{"", "", nil}, {"", "\n", []Hunk{{0, 0, 0, 1}}}, {"a\n", "a", []Hunk{{0, 1, 0, 1}}}, {"one\n\n", "one\n", []Hunk{{1, 2, 1, 1}}}} {
		if got := DiffLines(item.left, item.right).Hunks; !reflect.DeepEqual(got, item.want) {
			t.Errorf("diff %q / %q = %v, want %v", item.left, item.right, got, item.want)
		}
	}
}

func TestDiffLinesDenseChangesKeepUniqueAnchors(t *testing.T) {
	var left, right strings.Builder
	for index := range 2000 {
		fmt.Fprintf(&left, "left\nanchor %d\n", index)
		fmt.Fprintf(&right, "right\nanchor %d\n", index)
	}
	result := DiffLines(left.String(), right.String())
	if result.Approximate || len(result.Hunks) != 2000 || result.Hunks[1999] != (Hunk{3998, 3999, 3998, 3999}) {
		t.Fatalf("dense diff: %d hunks, approximate=%v", len(result.Hunks), result.Approximate)
	}
}

func TestDiffLinesReconstructsAndMinimizesSmallInputs(t *testing.T) {
	random := rand.New(rand.NewSource(42))
	for range 500 {
		var lines [2][]string
		for side := range lines {
			for range random.Intn(14) {
				lines[side] = append(lines[side], []string{"a\n", "b\n", "c\n", "d\n"}[random.Intn(4)])
			}
		}
		left, right := lines[0], lines[1]
		var rebuilt []string
		cost, leftAt, rightAt := 0, 0, 0
		for _, hunk := range DiffLines(strings.Join(left, ""), strings.Join(right, "")).Hunks {
			if hunk.LeftStart-leftAt != hunk.RightStart-rightAt {
				t.Fatalf("invalid hunk %v for %q / %q", hunk, left, right)
			}
			rebuilt = append(rebuilt, left[leftAt:hunk.LeftStart]...)
			rebuilt = append(rebuilt, right[hunk.RightStart:hunk.RightEnd]...)
			cost += hunk.LeftEnd - hunk.LeftStart + hunk.RightEnd - hunk.RightStart
			leftAt, rightAt = hunk.LeftEnd, hunk.RightEnd
		}
		rebuilt = append(rebuilt, left[leftAt:]...)
		row := make([]int, len(right)+1)
		for _, leftLine := range left {
			previous := 0
			for index, rightLine := range right {
				old := row[index+1]
				if leftLine == rightLine {
					row[index+1] = previous + 1
				} else {
					row[index+1] = max(row[index+1], row[index])
				}
				previous = old
			}
		}
		if !reflect.DeepEqual(rebuilt, right) || cost != len(left)+len(right)-2*row[len(right)] {
			t.Fatalf("diff %q / %q rebuilt %q, edit cost %d", left, right, rebuilt, cost)
		}
	}
}
