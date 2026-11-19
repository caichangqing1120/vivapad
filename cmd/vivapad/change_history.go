package main

import (
	"strings"

	"vivapad/internal/changehistory"
	"vivapad/internal/document"
)

const noChangeNavigationNotice = "No changes to navigate"

// H06: change-history navigation and Clear. Both consume the document's real
// consumer result; an unavailable history refuses with its reason instead of
// guessing "no changes".

// changeHistoryLineBounds returns the UTF-8 byte bounds LineMask needs for every
// zero-based logical line in one pass: content start, content end (before its
// EOL) and the next line's start. The final line without a trailing newline uses
// the value length for both end and next, which is the documented empty final line
// and EOF deletion-point case.
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

// navigateChange moves the caret to the next or previous changed line, skipping
// the current contiguous changed block and wrapping like the original
// changedHistoryGoTo. A document with no changed line leaves caret and selection
// untouched; an unavailable history or a failed line query reports the reason and
// never claims "no changes".
func (state *model) navigateChange(previous bool) {
	doc := state.space.Current()
	consumer, err := doc.ChangeHistory()
	if err != nil {
		state.notice = err.Error()
		return
	}
	value := doc.Editor.String()
	// One pass of line bounds: the predicate must not rescan the document for every
	// line it is asked about.
	starts, ends, nexts := changeHistoryLineBounds(value)
	count := len(starts)
	var queryErr error
	marked := func(line int) bool {
		if line < 0 || line >= count {
			return false
		}
		mask, maskErr := consumer.LineMask(starts[line], ends[line], nexts[line])
		if maskErr != nil {
			if queryErr == nil {
				queryErr = maskErr
			}
			return false
		}
		return mask != 0
	}
	current := strings.Count(value[:doc.Editor.ByteOffset(doc.Editor.Caret())], "\n")
	target, ok := changehistory.ChangedLine(current, count, previous, marked)
	if queryErr != nil {
		state.notice = queryErr.Error()
		return
	}
	if !ok {
		state.notice = noChangeNavigationNotice
		state.noChangeNoticeDoc = doc
		state.noChangeNoticeRevision = consumer.Revision()
		return
	}
	doc.GoToLine(target + 1)
	pane := state.activePane
	state.rectangles[pane] = rectangleSelection{}
	state.multiSelections[pane] = multiSelectionState{}
	state.selectAnchors[pane] = selectAnchor{}
	state.notice = ""
}

func (state *model) retireNoChangeNotice(replacement string) {
	state.noChangeNoticeDoc, state.noChangeNoticeRevision = nil, 0
	if state.notice == noChangeNavigationNotice {
		state.notice = replacement
	}
}

// clearChangeHistory forgets undo/redo and the change-history editions while
// preserving the document's real unsaved boundary and every non-history
// property. The document boundary refuses drafts, replaced editors and live
// compositions with an explicit error, which becomes the notice.
func (state *model) clearChangeHistory() {
	doc := state.space.Current()
	if err := doc.ClearChangeHistory(); err != nil {
		state.notice = err.Error()
		return
	}
	state.notice = "Change history cleared"
}

// changeHistoryNotice renders the explicit unavailable reason for menus that must
// not pretend the document has no changes.
func changeHistoryUnavailable(doc *document.Document) (string, bool) {
	if _, err := doc.ChangeHistory(); err != nil {
		return err.Error(), true
	}
	return "", false
}
