package main

import (
	"slices"
	"strconv"
	"strings"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

type styleMarkState struct {
	tracking indicatorTracker
	styles   [5][]text.Span
}

var styleColors = [5]gfx.Color{
	gfx.RGB(0, 255, 255), gfx.RGB(255, 128, 0), gfx.RGB(255, 255, 0), gfx.RGB(128, 0, 255), gfx.RGB(0, 128, 0),
}

func (state *model) styleSpans(doc *document.Document, style int) []text.Span {
	stored := state.styledMarks[doc]
	if stored == nil || style < 0 || style >= len(stored.styles) {
		return nil
	}
	if !stored.tracking.sync(doc.Editor, stored.styles[:]) {
		delete(state.styledMarks, doc)
		state.notice = "Styles cleared: edit history unavailable"
		return nil
	}
	return stored.styles[style]
}

func (state *model) styleToken(style int, all bool) {
	if style < 0 || style >= len(styleColors) {
		return
	}
	doc := state.space.Current()
	multi, rectangle := state.multiSelections[state.activePane], state.rectangles[state.activePane]
	if multi.active && multi.doc == doc && len(multi.ranges) > 1 || rectangle.active && rectangle.doc == doc {
		state.notice = "Choose one ordinary selection to style"
		return
	}
	selection, caret := doc.Editor.Selection(), doc.Editor.Caret()
	query := doc.SelectedSearchText(true)
	token := doc.Editor.Selection()
	if caret == selection.Start {
		doc.Editor.SetSelection(text.Range{Start: selection.End, End: selection.Start})
	} else {
		doc.Editor.SetSelection(selection)
	}
	if query == "" {
		state.notice = "Select a token to style"
		return
	}
	spans := []text.Span{{Start: doc.Editor.ByteOffset(token.Start), End: doc.Editor.ByteOffset(token.End)}}
	if all {
		var err error
		spans, err = doc.MatchesWith(query, document.SearchOptions{WholeWord: true})
		if err != nil {
			state.notice = err.Error()
			return
		}
	}
	previous := append([]text.Span(nil), state.styleSpans(doc, style)...)
	if state.styledMarks == nil {
		state.styledMarks = make(map[*document.Document]*styleMarkState)
	}
	stored := state.styledMarks[doc]
	if stored == nil {
		stored = &styleMarkState{tracking: trackIndicators(doc.Editor)}
		state.styledMarks[doc] = stored
	}
	stored.styles[style] = mergeMarkSpans(append(previous, spans...))
	state.notice = strconv.Itoa(len(spans)) + " occurrence(s) styled"
}

func (state *model) clearStyles(style int) {
	doc := state.space.Current()
	if style < 0 {
		delete(state.styledMarks, doc)
	} else if stored := state.styledMarks[doc]; stored != nil && style < len(stored.styles) {
		stored.styles[style] = nil
	}
	state.notice = "Styles cleared"
}

func (state *model) copyStyles(style int) {
	doc := state.space.Current()
	var spans []text.Span
	if style >= 0 {
		spans = state.styleSpans(doc, style)
	} else {
		for index := range styleColors {
			spans = append(spans, state.styleSpans(doc, index)...)
		}
		value := doc.Editor.String()
		slices.SortStableFunc(spans, func(left, right text.Span) int {
			if left.Start != right.Start {
				return left.Start - right.Start
			}
			return strings.Compare(value[left.Start:left.End], value[right.Start:right.End])
		})
	}
	if len(spans) == 0 {
		state.notice = "No styled text to copy"
		return
	}
	result, err := joinedMarkedText(doc.Editor.String(), spans)
	if err == nil {
		err = app.SetClipboardText(result)
	}
	if err != nil {
		state.notice = err.Error()
		return
	}
	state.notice = "Styled text copied"
}

func (state *model) navigateStyle(style int, previous bool) {
	doc := state.space.Current()
	spans := state.styleSpans(doc, style)
	if len(spans) == 0 {
		state.notice = "No marks in this style"
		return
	}
	navigateMarkSpans(doc, spans, previous)
}
