package main

import (
	"github.com/vivaui/viva/text"
)

func (state *model) matchBrace(selectBetween bool) {
	doc := state.space.Current()
	value := doc.Editor.String()
	caret := doc.Editor.ByteOffset(doc.Editor.Caret())
	open, close, ok := text.MatchBracket(value, caret)
	if !ok {
		return
	}
	if selectBetween {
		doc.Editor.SetSelection(text.Range{Start: doc.Editor.RuneIndex(open), End: doc.Editor.RuneIndex(close + 1)})
		return
	}
	partner := open
	if caret == open || caret > 0 && caret-1 == open {
		partner = close
	}
	doc.Editor.MoveTo(doc.Editor.RuneIndex(partner), false)
}
