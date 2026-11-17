package main

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"

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
	starts, ends, nexts := changeHistoryLineBounds(value)
	masks := make([]uint8, len(starts))
	for line := range starts {
		masks[line], err = consumer.LineMask(starts[line], ends[line], nexts[line])
		if err != nil {
			delete(state.changeView, doc)
			return nil
		}
	}
	colors := changeMarginColors()
	var bars []components.CodeEditorGutterBar
	for line, mask := range masks {
		for edition, color := range colors {
			bit := uint8(1 << edition)
			if mask&bit != 0 {
				bars = append(bars, components.CodeEditorGutterBar{
					Line: line, Fill: color, Stroke: color,
					JoinBefore: line > 0 && masks[line-1]&bit != 0,
					JoinAfter:  line+1 < len(masks) && masks[line+1]&bit != 0,
				})
			}
		}
	}
	runs, points, err := consumer.Decorations()
	if err != nil {
		delete(state.changeView, doc)
		return nil
	}
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
	margin := components.CodeEditorDecorations{GutterWidth: 9, GutterBackground: gfx.RGB(224, 224, 224), GutterBars: bars}
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

func (state *model) changeMarginLines(doc *document.Document) []components.CodeEditorGutterBar {
	if cached := state.changeDecorations(doc); cached != nil {
		return cached.margin.GutterBars
	}
	return nil
}

// changeMarginColors returns the palette for the margin lane. The original
// classic theme uses cyan for reverted text, green for saved, orange for modified
// and yellow-green for reverted-to-modified.
func changeMarginColors() [4]gfx.Color {
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
