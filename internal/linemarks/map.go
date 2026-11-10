package linemarks

import (
	"slices"
	"strings"
)

type Edit struct {
	Start, Removed, Inserted int
	LineStart, InsertedLF    bool
}

func Map(before, after string, marked []int, edits []Edit, complete bool) ([]int, bool) {
	if !complete || len(edits) == 0 && before != after {
		return nil, false
	}
	marked = slices.Clone(marked)
	slices.Sort(marked)
	points := make([]int, 0, len(marked))
	line, start := 0, 0
	for _, wanted := range marked {
		for line < wanted {
			newline := strings.IndexByte(before[start:], '\n')
			if newline < 0 {
				return nil, false
			}
			start += newline + 1
			line++
		}
		if wanted != line {
			return nil, false
		}
		points = append(points, start)
	}
	length := len(before)
	for _, edit := range edits {
		if edit.Start < 0 || edit.Removed < 0 || edit.Inserted < 0 ||
			edit.Removed > length || edit.Start > length-edit.Removed ||
			edit.Inserted > int(^uint(0)>>1)-(length-edit.Removed) || edit.InsertedLF && edit.Inserted == 0 {
			return nil, false
		}
		if edit.Removed > 0 && edit.Start == 0 && edit.Removed == length {
			points = nil
		} else {
			for index, point := range points {
				if point > edit.Start {
					points[index] = max(edit.Start, point-edit.Removed)
				}
			}
		}
		for index, point := range points {
			if point > edit.Start || point == edit.Start && edit.InsertedLF && edit.LineStart {
				points[index] += edit.Inserted
			}
		}
		length += edit.Inserted - edit.Removed
	}
	if length != len(after) {
		return nil, false
	}
	var mapped []int
	line, start = 0, 0
	for _, point := range points {
		if point < start || point > len(after) {
			return nil, false
		}
		line += strings.Count(after[start:point], "\n")
		start = point
		if len(mapped) == 0 || mapped[len(mapped)-1] != line {
			mapped = append(mapped, line)
		}
	}
	return mapped, true
}
