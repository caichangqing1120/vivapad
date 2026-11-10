package main

import (
	"strconv"
	"strings"

	"github.com/vivaui/viva/text"
	"vivapad/internal/byterange"
)

func (state *model) openByteRangeDialog() {
	if !state.byteRangeInitialized {
		state.byteRangeStart = text.NewEditor("0")
		state.byteRangeEnd = text.NewEditor("255")
		state.byteRangeInitialized = true
	}
	state.byteRangeOpen = true
	state.byteRangeError = ""
}

func (state *model) findByteRange() {
	if !state.byteRangeOpen {
		return
	}
	start, end := 128, 255
	switch state.byteRangeMode {
	case 1:
		start, end = 0, 127
	case 2:
		var startErr, endErr error
		start, startErr = strconv.Atoi(strings.TrimSpace(state.byteRangeStart.String()))
		end, endErr = strconv.Atoi(strings.TrimSpace(state.byteRangeEnd.String()))
		if startErr != nil || endErr != nil {
			state.byteRangeError = "Enter a byte range from 0 to 255"
			return
		}
	}
	doc := state.paneDocument(state.activePane)
	value := doc.Editor.String()
	offset, found, err := byterange.Find(value, doc.Editor.ByteOffset(doc.Editor.Caret()), start, end, state.byteRangeUp, state.byteRangeWrap)
	if err != nil {
		state.byteRangeError = err.Error()
		return
	}
	if !found {
		state.byteRangeError = "No character in range"
		return
	}
	first := doc.Editor.RuneIndex(offset)
	last := max(first+1, doc.Editor.RuneIndex(offset+1))
	if state.byteRangeUp {
		doc.Editor.SetSelection(text.Range{Start: last, End: first})
	} else {
		doc.Editor.SetSelection(text.Range{Start: first, End: last})
	}
	state.byteRangeError = ""
	state.codeEditorForPane(state.activePane).Reveal(doc.Editor.ByteOffset(last))
}
