package compare

import (
	"slices"
	"sort"
	"strings"
)

type Hunk struct {
	LeftStart, LeftEnd, RightStart, RightEnd int
}

func (hunk Hunk) Range(side int) (start, end int) {
	if side == 0 {
		return hunk.LeftStart, hunk.LeftEnd
	}
	return hunk.RightStart, hunk.RightEnd
}

type LineDiff struct {
	Hunks       []Hunk
	Approximate bool
}

func (diff LineDiff) PeerLine(side, line int) int {
	index := sort.Search(len(diff.Hunks), func(index int) bool {
		_, end := diff.Hunks[index].Range(side)
		return end > line
	})
	if index < len(diff.Hunks) {
		hunk := diff.Hunks[index]
		start, end := hunk.Range(side)
		peerStart, peerEnd := hunk.Range(1 - side)
		if line >= start {
			return peerStart + (line-start)*(peerEnd-peerStart)/(end-start)
		}
	}
	if index != 0 {
		_, end := diff.Hunks[index-1].Range(side)
		_, peerEnd := diff.Hunks[index-1].Range(1 - side)
		return peerEnd + line - end
	}
	return line
}

func DiffLines(left, right string) LineDiff {
	if left == right {
		return LineDiff{}
	}
	result := LineDiff{}
	work := 2_000_000
	result.diff(splitTextLines(left), splitTextLines(right), 0, 0, &work, 0)
	return result
}

func splitTextLines(value string) []string {
	lines := strings.SplitAfter(value, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

type linePair struct{ left, right int }

func (result *LineDiff) diff(left, right []string, leftBase, rightBase int, work *int, depth int) {
	for len(left) != 0 && len(right) != 0 && left[0] == right[0] {
		left, right = left[1:], right[1:]
		leftBase, rightBase = leftBase+1, rightBase+1
	}
	for len(left) != 0 && len(right) != 0 && left[len(left)-1] == right[len(right)-1] {
		left, right = left[:len(left)-1], right[:len(right)-1]
	}
	add := func(leftStart, leftEnd, rightStart, rightEnd int) {
		if leftStart != leftEnd || rightStart != rightEnd {
			result.Hunks = append(result.Hunks, Hunk{leftBase + leftStart, leftBase + leftEnd, rightBase + rightStart, rightBase + rightEnd})
		}
	}
	if len(left) == 0 || len(right) == 0 {
		add(0, len(left), 0, len(right))
		return
	}
	if pairs, exact := myersMatches(left, right, work); exact {
		leftAt, rightAt := 0, 0
		for _, pair := range pairs {
			add(leftAt, pair.left, rightAt, pair.right)
			leftAt, rightAt = pair.left+1, pair.right+1
		}
		add(leftAt, len(left), rightAt, len(right))
		return
	}
	anchors, shared := uniqueAnchors(left, right)
	if len(anchors) == 0 || depth >= 8 {
		result.Approximate = result.Approximate || shared
		add(0, len(left), 0, len(right))
		return
	}
	leftAt, rightAt := 0, 0
	for _, pair := range anchors {
		result.diff(left[leftAt:pair.left], right[rightAt:pair.right], leftBase+leftAt, rightBase+rightAt, work, depth+1)
		leftAt, rightAt = pair.left+1, pair.right+1
	}
	result.diff(left[leftAt:], right[rightAt:], leftBase+leftAt, rightBase+rightAt, work, depth+1)
}

func myersMatches(left, right []string, work *int) ([]linePair, bool) {
	const maxDistance = 512
	if max(len(left), len(right))-min(len(left), len(right)) > maxDistance {
		return nil, false
	}
	trace := make([][]int, 0, min(maxDistance+1, len(left)+len(right)+1))
	for distance := 0; distance <= min(maxDistance, len(left)+len(right)); distance++ {
		row := make([]int, distance+1)
		trace = append(trace, row)
		for diagonal := -distance; diagonal <= distance; diagonal += 2 {
			*work--
			if *work < 0 {
				return nil, false
			}
			leftAt := 0
			if distance > 0 {
				previous := trace[distance-1]
				from := previousDiagonal(previous, distance, diagonal)
				leftAt = previous[(from+distance-1)/2]
				if from < diagonal {
					leftAt++
				}
			}
			rightAt := leftAt - diagonal
			for leftAt < len(left) && rightAt < len(right) && left[leftAt] == right[rightAt] {
				leftAt, rightAt = leftAt+1, rightAt+1
				*work--
				if *work < 0 {
					return nil, false
				}
			}
			row[(diagonal+distance)/2] = leftAt
			if leftAt == len(left) && rightAt == len(right) {
				return traceMatches(trace, leftAt, rightAt), true
			}
		}
	}
	return nil, false
}

func previousDiagonal(row []int, distance, diagonal int) int {
	if diagonal == -distance || diagonal != distance && row[(diagonal-1+distance-1)/2] < row[(diagonal+1+distance-1)/2] {
		return diagonal + 1
	}
	return diagonal - 1
}

func traceMatches(trace [][]int, leftAt, rightAt int) []linePair {
	var pairs []linePair
	for distance := len(trace) - 1; distance >= 0; distance-- {
		beforeLeft, beforeRight := 0, 0
		if distance > 0 {
			previous := trace[distance-1]
			from := previousDiagonal(previous, distance, leftAt-rightAt)
			beforeLeft = previous[(from+distance-1)/2]
			beforeRight = beforeLeft - from
		}
		for leftAt > beforeLeft && rightAt > beforeRight {
			leftAt, rightAt = leftAt-1, rightAt-1
			pairs = append(pairs, linePair{leftAt, rightAt})
		}
		leftAt, rightAt = beforeLeft, beforeRight
	}
	slices.Reverse(pairs)
	return pairs
}

func uniqueAnchors(left, right []string) ([]linePair, bool) {
	type positions struct{ left, right int }
	counts := make(map[string]positions, len(left))
	for index, line := range left {
		entry := counts[line]
		if entry.left == 0 {
			entry.left = index + 1
		} else {
			entry.left = -1
		}
		counts[line] = entry
	}
	shared := false
	for index, line := range right {
		if entry, ok := counts[line]; ok {
			shared = true
			if entry.right == 0 {
				entry.right = index + 1
			} else {
				entry.right = -1
			}
			counts[line] = entry
		}
	}
	var tails, previous []int
	var pairs []linePair
	for index, line := range left {
		entry := counts[line]
		if entry.left != index+1 || entry.right <= 0 {
			continue
		}
		rightAt := entry.right - 1
		position := sort.Search(len(tails), func(index int) bool { return pairs[tails[index]].right >= rightAt })
		parent := -1
		if position > 0 {
			parent = tails[position-1]
		}
		if position == len(tails) {
			tails = append(tails, len(pairs))
		} else {
			tails[position] = len(pairs)
		}
		pairs = append(pairs, linePair{index, rightAt})
		previous = append(previous, parent)
	}
	anchors := make([]linePair, len(tails))
	if len(tails) > 0 {
		index := tails[len(tails)-1]
		for position := len(anchors) - 1; position >= 0; position-- {
			anchors[position], index = pairs[index], previous[index]
		}
	}
	return anchors, shared
}
