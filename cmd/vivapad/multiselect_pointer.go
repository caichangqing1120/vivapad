package main

import (
	"slices"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

type altEditorGesture struct {
	doc             *document.Document
	value           string
	pane            int
	origin          gfx.Point
	anchor, virtual int
	dragging        bool
}

func (state *model) altEditorPointer(event gfx.PointerEvent) gfx.EventResult {
	point := gfx.Point{X: event.X, Y: event.Y}
	if event.Kind == gfx.PointerDown {
		state.altEditorGesture = altEditorGesture{}
		if (event.Target != editorID && event.Target != rightEditorID) || event.Button != gfx.PointerButtonPrimary || event.Modifiers&gfx.ModifierAlt == 0 {
			return gfx.EventResult{}
		}
		pane := paneForTarget(event.Target)
		state.activatePane(pane)
		doc := state.paneDocument(pane)
		offset, virtual, ok := state.codeEditorForPane(pane).OffsetWithVirtualAt(point)
		if !ok || doc.Editor.CompositionActive() {
			return gfx.EventResult{}
		}
		state.altEditorGesture = altEditorGesture{doc: doc, value: doc.Editor.String(), pane: pane,
			origin: point, anchor: doc.Editor.RuneIndex(offset), virtual: virtual}
		return gfx.EventResult{Handled: true, Repaint: true, Focus: event.Target}
	}
	gesture := &state.altEditorGesture
	if gesture.doc == nil {
		return gfx.EventResult{}
	}
	if event.Kind == gfx.PointerExit || gesture.doc != state.paneDocument(gesture.pane) || gesture.value != gesture.doc.Editor.String() {
		*gesture = altEditorGesture{}
		return gfx.EventResult{}
	}
	if event.Kind != gfx.PointerMove && event.Kind != gfx.PointerUp {
		return gfx.EventResult{}
	}
	pane, doc := gesture.pane, gesture.doc
	deltaX, deltaY := point.X-gesture.origin.X, point.Y-gesture.origin.Y
	if gesture.dragging || deltaX*deltaX+deltaY*deltaY >= 9 {
		if offset, virtual, ok := state.codeEditorForPane(pane).OffsetWithVirtualAt(point); ok {
			if !gesture.dragging {
				state.multiSelections[pane] = multiSelectionState{}
				state.selectAnchors[pane] = selectAnchor{}
				gesture.dragging = true
			}
			caret := doc.Editor.RuneIndex(offset)
			doc.Editor.MoveTo(caret, false)
			state.rectangles[pane] = rectangleSelection{doc: doc, value: gesture.value, active: true,
				anchor: gesture.anchor, anchorVirtual: gesture.virtual, caret: caret, caretVirtual: virtual}
		}
	} else if event.Kind == gfx.PointerUp {
		state.toggleSecondaryCaret(pane, doc, gesture.anchor)
	}
	if event.Kind == gfx.PointerUp {
		*gesture = altEditorGesture{}
	}
	return gfx.EventResult{Handled: true, Repaint: true}
}

func (state *model) toggleSecondaryCaret(pane int, doc *document.Document, at int) {
	state.syncMultiSelection(pane, doc)
	selected := &state.multiSelections[pane]
	ranges := selected.ranges
	if !selected.active {
		ranges = []text.Range{doc.Editor.Selection()}
	}
	caret := text.Range{Start: at, End: at}
	for index, selection := range ranges {
		if selection == caret && index < len(ranges)-1 {
			if len(ranges) == 2 {
				*selected = multiSelectionState{}
			} else {
				selected.ranges = slices.Delete(ranges, index, index+1)
			}
			return
		}
		if selection.Start <= at && at <= selection.End {
			return
		}
	}
	state.rectangles[pane] = rectangleSelection{}
	state.selectAnchors[pane] = selectAnchor{}
	*selected = multiSelectionState{doc: doc, value: doc.Editor.String(), active: true,
		ranges: slices.Insert(ranges, len(ranges)-1, caret)}
	if state.macroRecording {
		state.cancelMacroUnsupported("Alt+Click")
	}
}
