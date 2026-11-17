package main

import (
	"strings"

	"vivapad/internal/changehistory"
)

func changeHistoryLineBounds(value string) (starts, ends, nexts []int) {
	count := strings.Count(value, "\n") + 1
	starts = make([]int, 0, count)
	ends = make([]int, 0, count)
	nexts = make([]int, 0, count)
	start := 0
	for {
		if newline := strings.IndexByte(value[start:], '\n'); newline >= 0 {
			starts = append(starts, start)
			ends = append(ends, start+newline)
			nexts = append(nexts, start+newline+1)
			start += newline + 1
			continue
		}
		starts = append(starts, start)
		ends = append(ends, len(value))
		nexts = append(nexts, len(value))
		return starts, ends, nexts
	}
}

func (state *model) navigateChange(previous bool) {
	doc := state.space.Current()
	consumer, err := doc.ChangeHistory()
	if err != nil {
		state.notice = err.Error()
		return
	}
	value := doc.Editor.String()
	starts, ends, nexts := changeHistoryLineBounds(value)
	var queryErr error
	marked := func(line int) bool {
		mask, err := consumer.LineMask(starts[line], ends[line], nexts[line])
		if err != nil {
			queryErr = err
			return false
		}
		return mask != 0
	}
	current := strings.Count(value[:doc.Editor.ByteOffset(doc.Editor.Caret())], "\n")
	target, found := changehistory.ChangedLine(current, len(starts), previous, marked)
	if queryErr != nil {
		state.notice = queryErr.Error()
		return
	}
	if !found {
		state.notice = "No changes to navigate"
		return
	}
	doc.GoToLine(target + 1)
	pane := state.activePane
	state.rectangles[pane] = rectangleSelection{}
	state.multiSelections[pane] = multiSelectionState{}
	state.selectAnchors[pane] = selectAnchor{}
	state.notice = ""
}

func (state *model) clearChangeHistory() {
	if err := state.space.Current().ClearChangeHistory(); err != nil {
		state.notice = err.Error()
		return
	}
	state.notice = "Change history cleared"
}
