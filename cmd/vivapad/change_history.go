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
	editor := doc.Editor
	value := editor.String()
	caret := editor.ByteOffset(editor.Caret())
	current := strings.Count(value[:caret], "\n")
	count := current + strings.Count(value[caret:], "\n") + 1
	line, start := current, strings.LastIndexByte(value[:caret], '\n')+1
	// ChangedLine walks neighboring lines in order, restarting at either end on wrap.
	seek := func(target int) {
		if line == target {
			return
		}
		switch target {
		case 0:
			line, start = 0, 0
		case count - 1:
			line, start = target, strings.LastIndexByte(value, '\n')+1
		default:
			for line < target {
				start += strings.IndexByte(value[start:], '\n') + 1
				line++
			}
			for line > target {
				start = strings.LastIndexByte(value[:start-1], '\n') + 1
				line--
			}
		}
	}
	var queryErr error
	marked := func(line int) bool {
		if line < 0 || line >= count {
			return false
		}
		seek(line)
		end, next := len(value), len(value)
		if newline := strings.IndexByte(value[start:], '\n'); newline >= 0 {
			end = start + newline
			next = end + 1
		}
		mask, maskErr := consumer.LineMask(start, end, next)
		if maskErr != nil {
			if queryErr == nil {
				queryErr = maskErr
			}
			return false
		}
		return mask != 0
	}
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
	seek(target)
	editor.MoveTo(editor.RuneIndex(start), false)
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
