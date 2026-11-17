package main

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"

	"vivapad/internal/changehistory"
	"vivapad/internal/document"
)

type changeViewCache struct {
	value     string
	consumer  *changehistory.Consumer
	revision  uint64
	margin    components.CodeEditorDecorations
	indicator components.CodeEditorDecorations
	both      components.CodeEditorDecorations
}

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
	if cached := state.changeView[doc]; cached != nil && cached.value == value && cached.consumer == consumer && cached.revision == consumer.Revision() {
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
	colors := [4]gfx.Color{palette.changeReverted, palette.changeSaved, palette.changeModified, palette.changeRevertedModified}
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
	both := margin
	both.IndicatorLayers = layers
	cached := &changeViewCache{
		value: value, consumer: consumer, revision: consumer.Revision(),
		margin: margin, indicator: components.CodeEditorDecorations{IndicatorLayers: layers}, both: both,
	}
	if state.changeView == nil {
		state.changeView = make(map[*document.Document]*changeViewCache)
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

func (state *model) applyChangeDecorations(snapshot *components.CodeEditorSnapshot, doc *document.Document) {
	snapshot.Decorations = nil
	if state.changeHistoryView == "off" {
		if doc != nil {
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
