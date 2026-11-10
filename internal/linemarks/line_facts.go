package linemarks

import (
	"slices"
	"strings"
)

type LineEdit struct {
	Start, Removed, Inserted          int
	Line, RemovedLines, InsertedLines int
	AtLineStart                       bool
}

func MapLines(before, after string, marked []int, edits []LineEdit, complete bool) ([]int, bool) {
	if !complete || len(edits) == 0 && before != after {
		return nil, false
	}
	lines := strings.Count(before, "\n") + 1
	if lines <= 0 {
		return nil, false
	}
	mapped := slices.Clone(marked)
	slices.Sort(mapped)
	for _, line := range mapped {
		if line < 0 || line >= lines {
			return nil, false
		}
	}
	length := len(before)
	maximum := int(^uint(0) >> 1)
	for _, edit := range edits {
		if edit.Start < 0 || edit.Removed < 0 || edit.Inserted < 0 || edit.Removed > length || edit.Start > length-edit.Removed ||
			edit.Inserted > maximum-(length-edit.Removed) || edit.Line < 0 || edit.Line >= lines || edit.Line > edit.Start ||
			edit.RemovedLines < 0 || edit.InsertedLines < 0 || edit.RemovedLines > edit.Removed || edit.InsertedLines > edit.Inserted ||
			edit.RemovedLines > lines-1-edit.Line || edit.InsertedLines > maximum-(lines-edit.RemovedLines) {
			return nil, false
		}
		if edit.Removed > 0 && edit.Start == 0 && edit.Removed == length {
			mapped = nil
		} else if edit.RemovedLines > 0 {
			for index, line := range mapped {
				if line > edit.Line {
					mapped[index] = max(edit.Line, line-edit.RemovedLines)
				}
			}
		}
		insertAt := edit.Line + 1
		if edit.AtLineStart {
			insertAt--
		}
		if edit.InsertedLines > 0 {
			for index, line := range mapped {
				if line >= insertAt {
					mapped[index] += edit.InsertedLines
				}
			}
		}
		length += edit.Inserted - edit.Removed
		lines += edit.InsertedLines - edit.RemovedLines
	}
	if length != len(after) || lines != strings.Count(after, "\n")+1 {
		return nil, false
	}
	return slices.Compact(mapped), true
}
