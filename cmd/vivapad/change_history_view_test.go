package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/text"

	"vivapad/internal/changehistory"
	"vivapad/internal/preferences"
)

// H05: the change margin is painted from the real consumer result.

func changeMarginDocument(t *testing.T, value string) *model {
	t.Helper()
	path := filepath.Join(t.TempDir(), "margin.txt")
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	state, _ := startDriver(t)
	if err := state.space.Open(path); err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	return state
}

func TestChangeMarginLinesFollowTheRealConsumer(t *testing.T) {
	state := changeMarginDocument(t, "\u4e00\r\ntw\u00f6\nthree\n")
	doc := state.space.Current()
	if lines := state.changeMarginLines(doc); len(lines) != 0 {
		t.Fatalf("a freshly opened document painted %v", lines)
	}
	doc.Editor.MoveTo(len([]rune("\u4e00\n")), false)
	doc.Editor.Insert("X")
	lines := state.changeMarginLines(doc)
	if len(lines) != 1 || lines[0].Line != 1 || lines[0].Fill != palette.changeModified {
		t.Fatalf("modified line decorations = %+v", lines)
	}
	// The cache must follow the text: another edit adds another decorated line.
	doc.Editor.MoveTo(len([]rune("\u4e00\nXtw\u00f6\n")), false)
	doc.Editor.Insert("Y")
	lines = state.changeMarginLines(doc)
	if len(lines) != 2 || lines[0].Line != 1 || lines[1].Line != 2 {
		t.Fatalf("two modified lines decorations = %+v", lines)
	}
	if lines[0].JoinBefore || !lines[0].JoinAfter || !lines[1].JoinBefore || lines[1].JoinAfter {
		t.Fatalf("adjacent lines did not join only along their shared edge: %+v", lines)
	}
	// Undoing everything returns to the saved bytes and must clear the margin.
	doc.Editor.Undo()
	doc.Editor.Undo()
	if lines := state.changeMarginLines(doc); len(lines) != 0 {
		t.Fatalf("restored document still paints %+v", lines)
	}
}

func TestChangeMarginHidesWhenHistoryIsUnavailable(t *testing.T) {
	state, _ := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("one\n")
	doc.Editor = text.NewEditor("replaced")
	if lines := state.changeMarginLines(doc); len(lines) != 0 {
		t.Fatalf("an unavailable consumer painted %+v", lines)
	}
}

func TestChangeMarginDeletionPoint(t *testing.T) {
	state := changeMarginDocument(t, "one\ntwo\nthree\n")
	doc := state.space.Current()
	doc.Editor.SetSelection(text.Range{Start: len([]rune("one\n")), End: len([]rune("one\ntwo"))})
	doc.Editor.DeleteBackward()
	lines := state.changeMarginLines(doc)
	var target *components.CodeEditorGutterBar
	for index := range lines {
		if lines[index].Line == 1 {
			target = &lines[index]
		}
	}
	if target == nil {
		t.Fatalf("deletion point missing from decorations: %+v", lines)
	}
	if target.Fill != palette.changeModified {
		t.Fatalf("deletion edition did not contribute to the margin: %+v", *target)
	}
}

func TestChangeMarginProjectsSparseRunsAndDeletionPoints(t *testing.T) {
	// Offsets 4 and 9 are LF bytes, 5 and 10 are line starts, and 25 is EOF.
	const value = "zero\none\n\ntwo\nquiet\nlast\n"
	runs := []changehistory.DecorationRun{
		{Start: 4, End: 6, Edition: 1},
		{Start: 9, End: 10, Edition: 2},
		{Start: 11, End: 13, Edition: 3},
		{Start: 20, End: 25, Edition: 3},
	}
	points := []changehistory.DeletionPoint{
		{Position: 4, Editions: 2},
		{Position: 5, Editions: 8},
		{Position: 9, Editions: 1},
		{Position: 10, Editions: 2},
		{Position: 25, Editions: 4},
	}
	colors := changeMarginColors()
	bar := func(line, edition int, before, after bool) components.CodeEditorGutterBar {
		color := colors[edition-1]
		return components.CodeEditorGutterBar{Line: line, Fill: color, Stroke: color, JoinBefore: before, JoinAfter: after}
	}
	want := []components.CodeEditorGutterBar{
		bar(0, 1, false, true), bar(0, 2, false, false),
		bar(1, 1, true, true), bar(1, 4, false, false),
		bar(2, 1, true, false), bar(2, 2, false, true),
		bar(3, 2, true, false), bar(3, 3, false, false),
		bar(5, 3, false, true), bar(6, 3, true, false),
	}
	if got := changeMarginBars(value, runs, points, colors); !slices.Equal(got, want) {
		t.Fatalf("change margin = %+v, want %+v", got, want)
	}
}

func TestChangeMarginCacheInvalidatesAfterClear(t *testing.T) {
	state := changeMarginDocument(t, "one\ntwo\n")
	doc := state.space.Current()
	doc.Editor.MoveTo(len([]rune("one\n")), false)
	doc.Editor.Insert("X")
	before := state.changeMarginLines(doc)
	if len(before) != 1 {
		t.Fatalf("precondition decorations = %+v", before)
	}
	if err := doc.ClearChangeHistory(); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if lines := state.changeMarginLines(doc); len(lines) != 0 {
		t.Fatalf("after clear the margin still paints %+v", lines)
	}
}

func TestChangeMarginCacheFollowsEqualTextEdits(t *testing.T) {
	state := changeMarginDocument(t, "one\ntwo\n")
	doc := state.space.Current()
	doc.Editor.MoveTo(len([]rune("one\n")), false)
	doc.Editor.Insert("X")
	before := state.changeMarginLines(doc)
	if len(before) != 1 {
		t.Fatalf("precondition decorations = %+v", before)
	}
	// A real edit that leaves the text unchanged still changes the editions; the
	// consumer revision must invalidate the cached decorations.
	doc.Editor.MoveTo(len([]rune("one\nXtwo")), false)
	doc.Editor.Insert("Y")
	doc.Editor.DeleteBackward()
	if text := doc.Editor.String(); text != "one\nXtwo\n" {
		t.Fatalf("precondition text = %q", text)
	}
	// The equal-text insert+delete leaves the bytes and the save truth unchanged
	// but adds a deletion point at the line's end; only a refreshed read sees it.
	state.changeHistoryView = "both"
	var snapshot components.CodeEditorSnapshot
	state.applyChangeDecorations(&snapshot, doc)
	decorations := actualChangeDecorations(t, snapshot)
	points := decorations.IndicatorLayers[6].Spans
	if !slices.Equal(points, []text.Span{{Start: 8, End: 8}}) {
		t.Fatalf("equal-text edit did not refresh the deletion points: %+v", points)
	}
}

func TestChangeMarginCacheIsReleasedWithTheDocument(t *testing.T) {
	state := changeMarginDocument(t, "one\ntwo\n")
	doc := state.space.Current()
	doc.Editor.MoveTo(len([]rune("one\n")), false)
	doc.Editor.Insert("X")
	if lines := state.changeMarginLines(doc); len(lines) != 1 {
		t.Fatalf("precondition decorations = %+v", lines)
	}
	if _, ok := state.changeView[doc]; !ok {
		t.Fatal("precondition: the document has no cached margin")
	}
	state.releaseDocument(doc)
	if _, ok := state.changeView[doc]; ok {
		t.Fatal("releasing the document kept its change margin cache")
	}
}

func TestChangeHistoryViewModesControlDecorations(t *testing.T) {
	state := changeMarginDocument(t, "one\ntwo\n")
	doc := state.space.Current()
	doc.Editor.MoveTo(len([]rune("one\n")), false)
	doc.Editor.Insert("X")
	for _, test := range []struct {
		view      string
		wantLines bool
		wantDots  bool
	}{
		{"margin", true, false},
		{"indicator", false, true},
		{"both", true, true},
		{"off", false, false},
	} {
		state.changeHistoryView = test.view
		snapshot := components.CodeEditorSnapshot{}
		state.applyChangeDecorations(&snapshot, doc)
		if test.view == "off" {
			if snapshot.Decorations != nil {
				t.Fatal("off retained decorations")
			}
			continue
		}
		decorations := actualChangeDecorations(t, snapshot)
		if (len(decorations.GutterBars) > 0) != test.wantLines {
			t.Fatalf("view %q bars = %d, want %v", test.view, len(decorations.GutterBars), test.wantLines)
		}
		if (len(decorations.IndicatorLayers) > 0) != test.wantDots {
			t.Fatalf("view %q indicators = %d, want %v", test.view, len(decorations.IndicatorLayers), test.wantDots)
		}
	}
}

func TestChangeHistoryViewSettingPersists(t *testing.T) {
	state := changeMarginDocument(t, "one\ntwo\n")
	// A driver without storage must report the failure instead of pretending to save.
	state.preferencesPath = ""
	state.command("Change History: Off")
	if !strings.Contains(state.notice, "settings storage unavailable") {
		t.Fatalf("unwritable settings notice = %q", state.notice)
	}
	if state.changeHistoryView != "off" {
		t.Fatalf("view = %q after an unwritable save, want the in-memory off", state.changeHistoryView)
	}
	state.preferencesPath = filepath.Join(t.TempDir(), "settings.json")
	state.command("Change History: Off")
	if state.changeHistoryView != "off" {
		t.Fatalf("view = %q, want off", state.changeHistoryView)
	}
	if state.notice != "Change history display off" {
		t.Fatalf("notice = %q", state.notice)
	}
	saved, err := preferences.Load(state.preferencesPath)
	if err != nil {
		t.Fatalf("reload preferences: %v", err)
	}
	if saved.ChangeHistoryView != "off" {
		t.Fatalf("stored view = %q, want an explicit off", saved.ChangeHistoryView)
	}
	state.command("Change History: Both")
	saved, err = preferences.Load(state.preferencesPath)
	if err != nil {
		t.Fatalf("reload preferences after both: %v", err)
	}
	if saved.ChangeHistoryView != "both" {
		t.Fatalf("stored view = %q, want both", saved.ChangeHistoryView)
	}
}

func TestChangeHistoryCommandsAreInTheMenus(t *testing.T) {
	for _, command := range []string{"Next Change", "Previous Change", "Clear Change History"} {
		found := false
		for _, menu := range menuCommands {
			if slices.Contains(menu, command) {
				found = true
			}
		}
		if !found {
			t.Fatalf("%q is not reachable from any menu", command)
		}
	}
	for _, command := range []string{"Change History: Margin", "Change History: Indicator", "Change History: Both", "Change History: Off"} {
		if !slices.Contains(menuCommands[6], command) {
			t.Fatalf("%q is missing from the Settings menu", command)
		}
	}
}

func TestChangeHistoryViewModesShowOneCheckMark(t *testing.T) {
	state := changeMarginDocument(t, "one\ntwo\n")
	for _, view := range []string{"margin", "indicator", "both", "off"} {
		state.changeHistoryView = view
		selected := 0
		for _, command := range []string{"Change History: Margin", "Change History: Indicator", "Change History: Both", "Change History: Off"} {
			if state.menuSelected(command) {
				selected++
			}
		}
		if selected != 1 {
			t.Fatalf("view %q selected %d entries, want exactly one", view, selected)
		}
	}
	state.changeHistoryView = ""
	if !state.menuSelected("Change History: Margin") {
		t.Fatal("an empty view must display as margin")
	}
}

func TestChangeHistoryViewNoticeSurvivesBookmarkSync(t *testing.T) {
	state := changeMarginDocument(t, "one\ntwo\n")
	state.preferencesPath = ""
	// Make the bookmark state incomplete so bookmarkLines writes its own notice
	// during the command's deferred sync; the settings error must win.
	state.setBookmarkLines(state.space.Current(), []int{0})
	if stored := state.bookmarks[state.space.Current()]; stored != nil {
		stored.incomplete = true
	}
	state.command("Change History: Off")
	if !strings.Contains(state.notice, "settings storage unavailable") {
		t.Fatalf("notice = %q, want the settings storage error", state.notice)
	}
}
