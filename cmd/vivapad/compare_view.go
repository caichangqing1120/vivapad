package main

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/compare"
	"vivapad/internal/document"
)

const (
	compareMenu         = "Compare"
	compareSplitCommand = "Compare Split Views"
	compareSavedCommand = "Compare with Saved"
	nextDiffCommand     = "Next Difference"
	previousDiffCommand = "Previous Difference"
	clearCompareCommand = "Clear Compare"
	compareLineLimit    = 200_000
)

var compareCommands = []string{compareSplitCommand, compareSavedCommand, nextDiffCommand, previousDiffCommand, clearCompareCommand}

var (
	comparePreviewID = tree.ID(page, "compare-saved-preview")
	compareScrollID  = tree.ID(page, "compare-saved-scrollbar")
	compareHorizID   = tree.ID(page, "compare-saved-horizontal")
)

type compareView struct {
	docs          [2]*document.Document
	values        [2]string
	starts        [2][]int
	result        compare.LineDiff
	current       int
	seen          [2]float32
	positionDirty bool
	reveal        bool
	saved         bool
	savedPane     int
	wasSplit      bool
	preview       components.CodeEditor
	palette       *themePalette
	marks         [2][]components.CodeEditorMarkLayer
	bars          [2][]components.CodeEditorGutterBar
}

func (state *model) compareCommand(command string) bool {
	switch command {
	case compareSplitCommand, compareSavedCommand:
		view := &compareView{current: -1, wasSplit: state.splitEnabled, positionDirty: true}
		if command == compareSplitCommand {
			if !state.splitEnabled || state.paneDocument(0) == state.paneDocument(1) {
				state.notice = "Open two different documents in Split View to compare them"
				return true
			}
			view.docs = [2]*document.Document{state.paneDocument(0), state.paneDocument(1)}
		} else {
			doc := state.space.Current()
			if doc.Path == "" {
				state.notice = "Save this document before comparing it with disk"
				return true
			}
			if !canCompareDocument(doc) {
				state.notice = "Compare is unavailable for large documents"
				return true
			}
			saved, err := document.ReadSearchSource(doc.Path, doc.Encoding(), document.LightweightThreshold-1)
			if err != nil {
				state.notice = "Compare with Saved: " + err.Error()
				return true
			}
			view.saved, view.savedPane = true, 1-state.activePane
			view.docs[1-view.savedPane] = doc
			view.values[view.savedPane] = saved.Text
		}
		if !view.refresh() {
			state.notice = "Compare is unavailable for large documents"
			return true
		}
		state.comparison = view
		state.notice = view.status()
	case nextDiffCommand, previousDiffCommand:
		view := state.activeComparison()
		if view == nil {
			state.notice = "No active comparison"
			return true
		}
		if len(view.result.Hunks) > 0 {
			count := len(view.result.Hunks)
			if command == nextDiffCommand {
				view.current = (view.current + 1) % count
			} else if view.current <= 0 {
				view.current = count - 1
			} else {
				view.current--
			}
			view.reveal = true
			for side, doc := range view.docs {
				if doc != nil {
					doc.Editor.MoveTo(doc.Editor.RuneIndex(view.hunkOffset(side)), false)
				}
			}
		}
		state.notice = view.status()
	case clearCompareCommand:
		state.clearComparison()
		state.notice = "Comparison cleared"
	default:
		return false
	}
	state.invalidator.Invalidate()
	return true
}

func (state *model) clearComparison() {
	if state.comparison != nil {
		state.comparison = nil
		state.syncNeedsCapture = true
	}
}

func canCompareDocument(doc *document.Document) bool {
	return doc != nil && doc.LargeFile() == nil && !doc.Lightweight() && int64(len(doc.Editor.String())) < document.LightweightThreshold
}

func (state *model) activeComparison() *compareView {
	view := state.comparison
	if view == nil {
		return nil
	}
	if state.distractionFree || state.splitEnabled != view.wasSplit ||
		view.saved && (state.activePane == view.savedPane || state.space.Current() != view.docs[1-view.savedPane]) ||
		!view.saved && view.docs != [2]*document.Document{state.paneDocument(0), state.paneDocument(1)} {
		state.clearComparison()
		state.notice = "Comparison cleared"
		return nil
	}
	return view
}

func (view *compareView) refresh() bool {
	values := view.values
	for side, doc := range view.docs {
		if doc != nil {
			if !canCompareDocument(doc) {
				return false
			}
			if doc.Editor.CompositionActive() {
				return true
			}
			values[side] = doc.Editor.String()
		}
	}
	if view.starts[0] != nil && values == view.values {
		return true
	}
	for _, value := range values {
		if strings.Count(value, "\n") >= compareLineLimit {
			return false
		}
	}
	view.values = values
	view.result = compare.DiffLines(values[0], values[1])
	for side, value := range values {
		starts := append(view.starts[side][:0], 0)
		for offset := 0; offset < len(value); offset++ {
			if value[offset] == '\n' {
				starts = append(starts, offset+1)
			}
		}
		view.starts[side] = starts
	}
	view.current, view.palette, view.positionDirty, view.reveal = -1, nil, true, false
	return true
}

func (view *compareView) status() string {
	count := len(view.result.Hunks)
	if count == 0 {
		return "Compare: files are identical"
	}
	message := fmt.Sprintf("Compare: %d difference blocks", count)
	if view.current >= 0 {
		message = fmt.Sprintf("Compare: difference %d of %d", view.current+1, count)
	}
	if view.result.Approximate {
		message += " (dense changes grouped)"
	}
	return message
}

func (view *compareView) offset(side, line int) int {
	if line >= len(view.starts[side]) {
		return len(view.values[side])
	}
	return view.starts[side][line]
}

func (view *compareView) hunkOffset(side int) int {
	start, _ := view.result.Hunks[view.current].Range(side)
	return view.offset(side, start)
}

func (view *compareView) style(palette *themePalette) {
	if view.palette == palette {
		return
	}
	view.palette = palette
	for side := range view.values {
		layers := make([]components.CodeEditorMarkLayer, len(palette.diff))
		for index, color := range palette.diff {
			layers[index].Color = color
			layers[index].Color.Alpha = 0.22
		}
		bars := view.bars[side][:0]
		for _, hunk := range view.result.Hunks {
			start, end := hunk.Range(side)
			peerStart, peerEnd := hunk.Range(1 - side)
			for line := start; line < end; line++ {
				kind := side
				if view.saved && view.savedPane == 1 {
					kind = 1 - side
				}
				if line-start < peerEnd-peerStart {
					kind = 2
				}
				color := palette.diff[kind]
				bars = append(bars, components.CodeEditorGutterBar{Line: line, Fill: color, Stroke: color})
				from, to := view.offset(side, line), view.offset(side, line+1)
				if from < to {
					layers[kind].Spans = append(layers[kind].Spans, text.Span{Start: from, End: to})
				}
			}
		}
		view.marks[side], view.bars[side] = layers, bars
	}
}

func (view *compareView) decorate(snapshot *components.CodeEditorSnapshot, side int, palette *themePalette) {
	if len(view.bars[side]) == 0 || snapshot.Value != view.values[side] {
		return
	}
	snapshot.MarkLayers = append(snapshot.MarkLayers, view.marks[side]...)
	decorations := components.CodeEditorDecorations{GutterWidth: 9, GutterBackground: palette.gutter, GutterBars: view.bars[side]}
	if current, ok := snapshot.Decorations.(*components.CodeEditorDecorations); ok {
		decorations.IndicatorLayers = current.IndicatorLayers
		decorations.GutterBars = append(slices.Clone(current.GutterBars), view.bars[side]...)
		sort.SliceStable(decorations.GutterBars, func(first, second int) bool {
			return decorations.GutterBars[first].Line < decorations.GutterBars[second].Line
		})
	}
	snapshot.Decorations = &decorations
}

func (state *model) paintComparison(canvas *gfx.Canvas, bounds gfx.Rect, view *compareView) {
	palette := state.palette()
	view.style(palette)
	position := state.splitPosition(bounds.Width)
	divider := gfx.Rect{X: bounds.X + position, Y: bounds.Y, Width: 8, Height: bounds.Height}
	canvas.Fill(divider, palette.tabs)
	canvas.Fill(gfx.Rect{X: divider.X + 3, Y: divider.Y, Width: 1, Height: divider.Height}, palette.border)
	if state.splitEnabled && !view.saved {
		canvas.HitWithOptions(splitterID, divider, "Resize editor views", gfx.HitOptions{Role: gfx.RoleSlider, Cursor: gfx.PointerCursorResizeHorizontal})
	}
	left := gfx.Rect{X: bounds.X, Y: bounds.Y, Width: position, Height: bounds.Height}
	right := gfx.Rect{X: divider.X + divider.Width, Y: bounds.Y, Width: bounds.Width - position - 8, Height: bounds.Height}
	for side, pane := range [2]gfx.Rect{left, right} {
		var label string
		if doc := view.docs[side]; doc != nil {
			label = doc.Name
			if view.saved {
				label += " (Current)"
			}
		} else {
			label = view.docs[1-side].Name + " (Saved, Read-Only)"
		}
		header := gfx.Rect{X: pane.X, Y: pane.Y, Width: pane.Width, Height: 23}
		canvas.Border(header, 1, palette.border, palette.tabs)
		smallLabel(canvas, canvas.FitLine(label, 11, gfx.WeightRegular, max(0, header.Width-16)), header.Inset(gfx.All(5)), palette.text, 11)
		pane.Y, pane.Height = pane.Y+header.Height, pane.Height-header.Height
		if doc := view.docs[side]; doc != nil {
			state.paintPane(canvas, pane, side, doc)
		} else {
			theme := state.editorTheme()
			snapshot := components.CodeEditorSnapshot{ID: comparePreviewID, Label: "Saved version (Read-Only)", ReadOnly: true,
				Value: view.values[side], FontSize: float32(state.paneFontSize(1 - side)), TabWidth: state.tabWidth,
				LineNumbers: state.showLineNumbers, Wrap: state.wrap, ScrollbarID: compareScrollID, HorizontalScrollbarID: compareHorizID}
			view.decorate(&snapshot, side, palette)
			view.preview.Paint(canvas, theme, pane, snapshot)
		}
	}
	state.syncCompareAfterPaint(view)
	if !view.saved {
		state.syncScrollAfterPaint()
	}
}

func (state *model) compareEditor(view *compareView, side int) *components.CodeEditor {
	if view.saved && side == view.savedPane {
		return &view.preview
	}
	return state.codeEditorForPane(side)
}

func (state *model) syncCompareFrom(view *compareView, side int) {
	if len(view.starts[0]) == 0 || len(view.starts[1]) == 0 {
		return
	}
	source, peer := state.compareEditor(view, side), state.compareEditor(view, 1-side)
	if row, ok := source.VisualRowAtY(source.ScrollY()); ok {
		line := sort.SearchInts(view.starts[side], row.Start+1) - 1
		peerLine := view.result.PeerLine(side, max(0, line))
		peerOffset := view.offset(1-side, peerLine)
		if offset := view.starts[side][max(0, line)]; row.Start > offset && view.offset(1-side, peerLine+1) > peerOffset {
			peerOffset += min(row.Start-offset, view.offset(1-side, peerLine+1)-peerOffset-1)
		}
		if position, ok := peer.YAtOffset(peerOffset); ok {
			peer.SetScrollY(position)
		}
	}
	view.seen[side], view.seen[1-side] = source.ScrollY(), peer.ScrollY()
}

func (state *model) syncCompareAfterPaint(view *compareView) {
	left, right := state.compareEditor(view, 0), state.compareEditor(view, 1)
	before := [2]float32{left.ScrollY(), right.ScrollY()}
	if view.reveal && view.current >= 0 {
		for side, editor := range [2]*components.CodeEditor{left, right} {
			if target, ok := editor.YAtOffset(view.hunkOffset(side)); ok {
				editor.SetScrollY(target)
			}
		}
		view.reveal = false
	} else if view.positionDirty || before != view.seen {
		side := state.activePane
		if !view.positionDirty && before[side] == view.seen[side] {
			side = 1 - side
		}
		state.syncCompareFrom(view, side)
	}
	view.positionDirty = false
	view.seen = [2]float32{left.ScrollY(), right.ScrollY()}
	if view.seen != before {
		state.invalidator.Invalidate()
	}
}

func isComparePreview(target gfx.ID) bool {
	return target == comparePreviewID || target == compareScrollID || target == compareHorizID
}
