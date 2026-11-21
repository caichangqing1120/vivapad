package changehistory

import (
	"fmt"
	"strings"
)

// LineMask returns Scintilla's four low edition bits for one logical line:
// reverted to origin, saved, modified, and reverted to modified. The caller
// supplies UTF-8 byte offsets for its content start/end and next line start.
// Inserted EOL bytes belong to this line; deletion points stop at its content
// end, inclusively. An empty final line can therefore have a deletion marker.
// The projection follows Document::GetMark; see SCINTILLA-LICENSE.txt.
func (tracker *Tracker) LineMask(start, end, next int) (uint8, error) {
	if start < 0 || end < start || next < end || next > tracker.Length() {
		return 0, fmt.Errorf("invalid change-history line bounds %d,%d,%d for length %d", start, end, next, tracker.Length())
	}
	var mask uint8
	for position := start; position < next; position = tracker.EndRun(position) {
		if edition := tracker.EditionAt(position); edition != 0 {
			mask |= 1 << (edition - 1)
		}
	}
	for position := start; position <= end; position = tracker.NextDelete(position) {
		mask |= uint8(tracker.DeletesAt(position))
	}
	return mask, nil
}

// ChangedLine follows Notepad++ changedHistoryGoTo: skip the current contiguous
// changed block, then wrap forward to the first changed line or backward to the
// last. A wrapped target may be in the current block. marked must be a pure
// query of the current line's change-history bits, independent of bookmarks.
// Invalid arguments and a document with no changes return (-1, false).
func ChangedLine(current, lineCount int, previous bool, marked func(int) bool) (int, bool) {
	if current < 0 || current >= lineCount || marked == nil {
		return -1, false
	}
	if previous {
		line := current
		for line >= 0 && marked(line) {
			line--
		}
		for ; line >= 0; line-- {
			if marked(line) {
				return line, true
			}
		}
		for line := lineCount - 1; line >= 0; line-- {
			if marked(line) {
				return line, true
			}
		}
	} else {
		block := current
		for line := current; line < lineCount; line++ {
			if marked(line) {
				if line != block {
					return line, true
				}
				block++
			}
		}
		for line := 0; line <= current; line++ {
			if marked(line) {
				return line, true
			}
		}
	}
	return -1, false
}

// ChangedLineOffset applies ChangedLine's navigation rules to the sorted runs
// and points from Decorations. It returns the target line's byte offset in
// LF-normalized text without visiting the untouched lines.
func ChangedLineOffset(value string, caret int, previous bool, runs []DecorationRun, points []DeletionPoint) (int, bool) {
	if caret < 0 || caret > len(value) || len(runs) == 0 && len(points) == 0 {
		return -1, false
	}
	lineStart := func(offset int) int { return strings.LastIndexByte(value[:offset], '\n') + 1 }
	current := lineStart(caret)
	first, last, before, after := -1, -1, -1, -1
	blockStart, blockLast, blockNext := -1, -1, 0
	finishBlock := func() {
		if blockStart < 0 {
			return
		}
		if first < 0 {
			first = blockStart
		}
		last = blockLast
		if blockLast < current {
			before = blockLast
		}
		if after < 0 && blockStart > current {
			after = blockStart
		}
	}
	for len(runs) != 0 || len(points) != 0 {
		var low, high int // inclusive byte positions; LF belongs to the preceding line
		if len(points) != 0 && (len(runs) == 0 || points[0].Position < runs[0].Start) {
			low, high = points[0].Position, points[0].Position
			points = points[1:]
		} else {
			low, high = runs[0].Start, runs[0].End-1
			runs = runs[1:]
		}
		if high < blockNext {
			continue
		}
		if low >= blockNext {
			start := lineStart(low)
			if blockStart < 0 || start > blockNext {
				finishBlock()
				blockStart = start
			}
		}
		blockLast = lineStart(high)
		blockNext = len(value) + 1 // Distinguish the real empty line after a trailing LF.
		if newline := strings.IndexByte(value[high:], '\n'); newline >= 0 {
			blockNext = high + newline + 1
		}
	}
	finishBlock()
	if previous {
		if before >= 0 {
			return before, true
		}
		return last, last >= 0
	}
	if after >= 0 {
		return after, true
	}
	return first, first >= 0
}
