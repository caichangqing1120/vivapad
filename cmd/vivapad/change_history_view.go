package main

import (
	"strings"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"

	"vivapad/internal/changehistory"
	"vivapad/internal/document"
)

// H05: the change margin is painted from the real consumer result. The app owns
// only the translation from edition/deletion bits to lane decorations; the
// editor component owns the lane geometry.

func (state *model) changeDecorations(doc *document.Document) *changeViewCache {
	if doc == nil || doc.Editor == nil {
		return nil
	}
	consumer, err := doc.ChangeHistory()
	if err != nil {
		delete(state.changeView, doc)
		return nil
	}
	value := doc.Editor.String()
	atSave := doc.Editor.AtSavePoint()
	revision := consumer.Revision()
	cached, ok := state.changeView[doc]
	if ok && cached.value == value && cached.consumer == consumer && cached.atSave == atSave && cached.revision == revision {
		return cached
	}
	runs, points, err := consumer.Decorations()
	if err != nil {
		delete(state.changeView, doc)
		return nil
	}
	colors := state.palette().changeMarginColors()
	bars := changeMarginBars(value, runs, points, colors)
	layers := make([]components.CodeEditorIndicatorLayer, 8)
	for edition, color := range colors {
		layers[edition].Color = color
		layers[edition].Color.Alpha *= 50.0 / 255.0
		layers[edition+4].Color = color
		layers[edition+4].Style = components.CodeEditorPointBelow
		if edition == 2 {
			layers[edition+4].Style = components.CodeEditorPointAbove
		}
	}
	for _, run := range runs {
		layer := &layers[run.Edition-1]
		layer.Spans = append(layer.Spans, text.Span{Start: run.Start, End: run.End})
	}
	for _, point := range points {
		for edition := range colors {
			if point.Editions&(1<<edition) != 0 {
				layer := &layers[edition+4]
				layer.Spans = append(layer.Spans, text.Span{Start: point.Position, End: point.Position})
			}
		}
	}
	margin := components.CodeEditorDecorations{GutterWidth: 9, GutterBackground: state.palette().gutter, GutterBars: bars}
	combined := margin
	combined.IndicatorLayers = layers
	cached = &changeViewCache{
		value: value, consumer: consumer, atSave: atSave, revision: revision,
		margin: margin, indicator: components.CodeEditorDecorations{IndicatorLayers: layers}, both: combined,
	}
	if state.changeView == nil {
		state.changeView = newChangeViewCache()
	}
	state.changeView[doc] = cached
	return cached
}

func changeMarginBars(value string, runs []changehistory.DecorationRun, points []changehistory.DeletionPoint, colors [4]gfx.Color) []components.CodeEditorGutterBar {
	var bars []components.CodeEditorGutterBar
	line, start := 0, 0
	previousLine, previousBar, previousMask := -2, 0, uint8(0)
	for len(runs) != 0 || len(points) != 0 {
		first := len(value)
		if len(runs) != 0 {
			first = runs[0].Start
		}
		if len(points) != 0 {
			first = min(first, points[0].Position)
		}
		// An edition can span several lines. In that case resume at the
		// current line start, otherwise skip the untouched bytes in one count.
		first = max(start, first)
		line += strings.Count(value[start:first], "\n")
		end, next := len(value), len(value)
		if newline := strings.IndexByte(value[first:], '\n'); newline != -1 {
			end = first + newline
			next = end + 1
		}
		var mask uint8
		// LineMask assigns inserted LF bytes to this line, as well as deletion
		// points at its content end. At EOF an empty line can own a point.
		for len(runs) != 0 && runs[0].Start < next {
			mask |= 1 << (runs[0].Edition - 1)
			if runs[0].End > next {
				break
			}
			runs = runs[1:]
		}
		for len(points) != 0 && points[0].Position <= end {
			mask |= points[0].Editions
			points = points[1:]
		}
		if previousLine != line-1 {
			previousMask = 0
		}
		firstBar := len(bars)
		for edition, color := range colors {
			bit := uint8(1 << edition)
			joined := previousMask&bit != 0
			if mask&bit != 0 {
				if joined {
					bars[previousBar].JoinAfter = true
				}
				bars = append(bars, components.CodeEditorGutterBar{Line: line, Fill: color, Stroke: color, JoinBefore: joined})
			}
			if joined {
				previousBar++
			}
		}
		previousLine, previousBar, previousMask = line, firstBar, mask
		line, start = line+1, next
	}
	return bars
}

func (state *model) changeMarginLines(doc *document.Document) []components.CodeEditorGutterBar {
	if cached := state.changeDecorations(doc); cached != nil {
		return cached.margin.GutterBars
	}
	return nil
}

// changeMarginColors returns the palette for the margin lane. The original
// classic theme uses cyan for reverted text, green for saved, orange for modified
// and yellow-green for reverted-to-modified.
func (palette *themePalette) changeMarginColors() [4]gfx.Color {
	return [4]gfx.Color{
		palette.changeReverted,
		palette.changeSaved,
		palette.changeModified,
		palette.changeRevertedModified,
	}
}

// forgetChangeView drops the cached margin after an operation that changes the
// text or the history without changing the consumer.
func (state *model) forgetChangeView(doc *document.Document) {
	delete(state.changeView, doc)
}

// setChangeHistoryView selects how the change history is drawn and persists the
// setting. An unavailable or unwritable preferences file is reported, never
// silently ignored.
func (state *model) setChangeHistoryView(view string) {
	state.changeHistoryView = view
	for doc := range state.changeView {
		delete(state.changeView, doc)
	}
	if err := state.savePreferences(); err != nil {
		state.notice = "Change history view: " + err.Error()
		return
	}
	switch view {
	case "margin":
		state.notice = "Change history shown in the margin"
	case "indicator":
		state.notice = "Change history shown as indicators"
	case "both":
		state.notice = "Change history shown in the margin and as indicators"
	case "off":
		state.notice = "Change history display off"
	}
}

// applyChangeDecorations puts the document's change decorations on the snapshot
// according to the selected display mode. An unavailable history contributes
// nothing: the margin and the indicators both stay hidden.
func (state *model) applyChangeDecorations(snapshot *components.CodeEditorSnapshot, doc *document.Document) {
	snapshot.Decorations = nil
	if state.changeHistoryView == "off" {
		// Visibility must not let the bounded producer ring outrun its consumer.
		if doc != nil && doc.Editor != nil {
			if _, err := doc.ChangeHistory(); err != nil {
				delete(state.changeView, doc)
			}
		}
		return
	}
	cached := state.changeDecorations(doc)
	if cached == nil {
		return
	}
	switch state.changeHistoryView {
	case "indicator":
		snapshot.Decorations = &cached.indicator
	case "both":
		snapshot.Decorations = &cached.both
	default:
		snapshot.Decorations = &cached.margin
	}
}
