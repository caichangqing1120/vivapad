package main

import (
	"reflect"
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func actualChangeDecorations(t *testing.T, snapshot components.CodeEditorSnapshot) *components.CodeEditorDecorations {
	t.Helper()
	decorations, ok := snapshot.Decorations.(*components.CodeEditorDecorations)
	if !ok || decorations == nil {
		t.Fatal("snapshot has no generic change decorations")
	}
	return decorations
}

func TestChangeDecorationsReserveEmptyLaneAndReuseCache(t *testing.T) {
	state := changeMarginDocument(t, "a\n")
	doc := state.space.Current()
	for _, mode := range []string{"margin", "both", "indicator", "off"} {
		state.changeHistoryView = mode
		var snapshot components.CodeEditorSnapshot
		state.applyChangeDecorations(&snapshot, doc)
		if mode == "off" {
			if snapshot.Decorations != nil {
				t.Fatal("off retained decoration data")
			}
			continue
		}
		decorations := actualChangeDecorations(t, snapshot)
		width := float32(9)
		if mode == "indicator" {
			width = 0
		}
		if decorations.GutterWidth != width || len(decorations.GutterBars) != 0 {
			t.Fatalf("mode=%s: empty lane = %+v", mode, decorations)
		}
		if width > 0 && decorations.GutterBackground != gfx.RGB(224, 224, 224) {
			t.Fatal("margin background differs from the original classic theme")
		}
		if allocations := testing.AllocsPerRun(50, func() { state.applyChangeDecorations(&snapshot, doc) }); allocations != 0 {
			t.Fatalf("mode=%s: cached empty decorations allocate %v times", mode, allocations)
		}
	}
}

func TestChangeDecorationsUseExactByteRunsAndEditionPoints(t *testing.T) {
	state := changeMarginDocument(t, "a\u6f22b\n")
	state.changeHistoryView = "both"
	doc := state.space.Current()
	doc.Editor.MoveTo(1, false)
	doc.Editor.Insert("\u00e9")
	doc.Editor.MoveTo(3, false)
	doc.Editor.DeleteForward()
	var snapshot components.CodeEditorSnapshot
	state.applyChangeDecorations(&snapshot, doc)
	decorations := actualChangeDecorations(t, snapshot)
	if len(decorations.IndicatorLayers) != 8 {
		t.Fatalf("indicator layers = %d, want four run and four point layers", len(decorations.IndicatorLayers))
	}
	run, point := decorations.IndicatorLayers[2], decorations.IndicatorLayers[6]
	color := classicPalette.changeModified
	color.Alpha *= 50.0 / 255.0
	if run.Style != components.CodeEditorUnderline || run.Color != color || !reflect.DeepEqual(run.Spans, []text.Span{{Start: 1, End: 3}}) {
		t.Fatalf("modified UTF-8 run = %+v", run)
	}
	if point.Style != components.CodeEditorPointAbove || point.Color != classicPalette.changeModified || !reflect.DeepEqual(point.Spans, []text.Span{{Start: 6, End: 6}}) {
		t.Fatalf("modified deletion = %+v", point)
	}
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	state.applyChangeDecorations(&snapshot, doc)
	decorations = actualChangeDecorations(t, snapshot)
	if got := decorations.IndicatorLayers[1].Spans; !reflect.DeepEqual(got, run.Spans) {
		t.Fatalf("saved run = %+v, want %+v", got, run.Spans)
	}
	point = decorations.IndicatorLayers[5]
	if point.Style != components.CodeEditorPointBelow || point.Color != classicPalette.changeSaved || !reflect.DeepEqual(point.Spans, []text.Span{{Start: 6, End: 6}}) {
		t.Fatalf("saved deletion = %+v", point)
	}
	if len(decorations.IndicatorLayers[2].Spans) != 0 || len(decorations.IndicatorLayers[6].Spans) != 0 {
		t.Fatal("save retained stale modified metadata")
	}
}

func TestChangeDecorationsLayerMarginUsingFullNeighborMasks(t *testing.T) {
	state := changeMarginDocument(t, "a\nb\n")
	doc := state.space.Current()
	doc.Editor.MoveTo(0, false)
	doc.Editor.Insert("S")
	doc.Editor.MoveTo(3, false)
	doc.Editor.Insert("T")
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	doc.Editor.MoveTo(0, false)
	doc.Editor.Insert("M")
	var snapshot components.CodeEditorSnapshot
	state.applyChangeDecorations(&snapshot, doc)
	decorations := actualChangeDecorations(t, snapshot)
	want := []components.CodeEditorGutterBar{
		{Line: 0, Fill: classicPalette.changeSaved, Stroke: classicPalette.changeSaved, JoinAfter: true},
		{Line: 0, Fill: classicPalette.changeModified, Stroke: classicPalette.changeModified},
		{Line: 1, Fill: classicPalette.changeSaved, Stroke: classicPalette.changeSaved, JoinBefore: true},
	}
	if !reflect.DeepEqual(decorations.GutterBars, want) {
		t.Fatalf("layered bars = %+v, want %+v", decorations.GutterBars, want)
	}
}

func TestChangeDecorationsHideInvalidatedCacheBeforeReuse(t *testing.T) {
	state := changeMarginDocument(t, "a\n")
	doc := state.space.Current()
	doc.Editor.Insert("x")
	var snapshot components.CodeEditorSnapshot
	state.applyChangeDecorations(&snapshot, doc)
	actualChangeDecorations(t, snapshot)
	doc.Editor.ClearHistory()
	state.applyChangeDecorations(&snapshot, doc)
	if snapshot.Decorations != nil {
		t.Fatal("lost producer checkpoint reused stale cached metadata")
	}
}

func TestChangeDecorationsOffStillConsumesEveryPaintedFrame(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	state.changeHistoryView = "off"
	clickID(t, driver, editorID)
	for index := 0; index < 300; index++ {
		if err := driver.Type("x"); err != nil {
			t.Fatal(err)
		}
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
	}
	state.changeHistoryView = "both"
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if _, err := doc.ChangeHistory(); err != nil {
		t.Fatalf("turning display off lost the producer checkpoint: %v", err)
	}
	var snapshot components.CodeEditorSnapshot
	state.applyChangeDecorations(&snapshot, doc)
	decorations := actualChangeDecorations(t, snapshot)
	if len(decorations.GutterBars) != 1 || len(decorations.IndicatorLayers[2].Spans) == 0 {
		t.Fatal("turning display back on lost the real edits")
	}
}

func TestChangeDecorationsKeepFinalEmptyLineDeletionInEveryMode(t *testing.T) {
	state := changeMarginDocument(t, "a\nx")
	doc := state.space.Current()
	doc.Editor.MoveTo(3, false)
	doc.Editor.DeleteBackward()
	if doc.Editor.String() != "a\n" {
		t.Fatal("fixture did not leave a final empty line")
	}
	for _, mode := range []string{"margin", "indicator", "both"} {
		state.changeHistoryView = mode
		var snapshot components.CodeEditorSnapshot
		state.applyChangeDecorations(&snapshot, doc)
		decorations := actualChangeDecorations(t, snapshot)
		if mode != "indicator" {
			if len(decorations.GutterBars) != 1 || decorations.GutterBars[0].Line != 1 || decorations.GutterBars[0].Fill != classicPalette.changeModified {
				t.Fatalf("mode=%s: final empty line bars = %+v", mode, decorations.GutterBars)
			}
		}
		if mode != "margin" && !reflect.DeepEqual(decorations.IndicatorLayers[6].Spans, []text.Span{{Start: 2, End: 2}}) {
			t.Fatalf("mode=%s: EOF deletion = %+v", mode, decorations.IndicatorLayers[6].Spans)
		}
	}
}
