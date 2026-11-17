package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func openChangeHistoryFixture(t *testing.T, value string) (*model, *app.Driver) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "changes.txt")
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
	state, driver := startDriver(t)
	if err := state.space.Open(path); err != nil {
		t.Fatal(err)
	}
	return state, driver
}

func TestChangeHistorySearchSubmenuExposesAndRunsCommands(t *testing.T) {
	state, driver := openChangeHistoryFixture(t, "one\ntwo\nthree\n")
	doc := state.space.Current()
	doc.Editor.MoveTo(4, false)
	doc.Editor.Insert("X")
	doc.Editor.MoveTo(0, false)
	state.command("Change History")
	if !state.changeHistoryMenuOpen || state.menuOpen != 2 {
		t.Fatal("Change History did not open the Search submenu")
	}
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	for index, label := range []string{"Go to Next Change", "Go to Previous Change", "Clear Change History"} {
		found := false
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == changeHistoryScope.At(index) && hit.Label == label && !hit.Disabled {
				found = true
			}
		}
		if !found {
			t.Fatalf("submenu entry %q is missing or disabled", label)
		}
	}
	clickID(t, driver, changeHistoryScope.At(0))
	if doc.Editor.Caret() != 4 || state.changeHistoryMenuOpen || state.menuOpen >= 0 {
		t.Fatalf("submenu did not navigate or close: caret=%d menu=%v", doc.Editor.Caret(), state.changeHistoryMenuOpen)
	}
}

func TestChangeHistorySearchMenuReachableAtCompactSize(t *testing.T) {
	state, driver := openChangeHistoryFixture(t, "one\ntwo\n")
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	index := slices.Index(menuCommands[2][:], "Change History")
	if index < 0 {
		t.Fatal("Search menu has no Change History entry")
	}
	clickID(t, driver, menuScope.At(2))
	for step := 0; step < len(menuCommands[2]); step++ {
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		visible := false
		panel := gfx.Rect{}
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == menuItemScope.At(index) {
				visible = true
			}
			if hit.ID == searchMenuPanelID {
				panel = hit.Bounds
			}
		}
		if visible {
			break
		}
		if panel.Width == 0 {
			t.Fatal("Search menu panel is missing")
		}
		if err := driver.Scroll(gfx.ScrollEvent{X: panel.X + panel.Width/2, Y: panel.Y + panel.Height/2, DeltaY: 60}); err != nil {
			t.Fatal(err)
		}
	}
	clickID(t, driver, menuItemScope.At(index))
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if !state.changeHistoryMenuOpen {
		t.Fatal("compact Search menu did not open Change History")
	}
	for subindex := range changeHistoryItems {
		found := false
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == changeHistoryScope.At(subindex) && hit.Bounds.X >= 0 && hit.Bounds.Y >= 0 && hit.Bounds.Right() <= 640 && hit.Bounds.Bottom() <= 400 {
				found = true
			}
		}
		if !found {
			t.Fatalf("compact submenu entry %d is outside the window", subindex)
		}
	}
}

func TestChangeHistorySubmenuKeyboardNavigationAndUnavailableState(t *testing.T) {
	state, driver := openChangeHistoryFixture(t, "one\ntwo\n")
	doc := state.space.Current()
	doc.Editor.MoveTo(4, false)
	doc.Editor.Insert("X")
	parent := menuItemScope.At(slices.Index(menuCommands[2][:], "Change History"))
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, parent)
	for _, item := range []struct {
		key  gfx.Key
		want int
	}{{gfx.KeyDown, 0}, {gfx.KeyDown, 1}, {gfx.KeyDown, 2}, {gfx.KeyDown, 0}, {gfx.KeyUp, 2}} {
		if err := driver.Key(item.key, 0); err != nil {
			t.Fatal(err)
		}
		if driver.Snapshot().Focused != changeHistoryScope.At(item.want) {
			t.Fatalf("key %v focused %v, want %d", item.key, driver.Snapshot().Focused, item.want)
		}
	}
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if state.changeHistoryMenuOpen || state.menuOpen != 2 || driver.Snapshot().Focused != parent {
		t.Fatal("Escape did not return to the Search parent")
	}
	if err := driver.Key(gfx.KeyRight, 0); err != nil {
		t.Fatal(err)
	}
	if !state.changeHistoryMenuOpen || driver.Snapshot().Focused != changeHistoryScope.At(0) {
		t.Fatal("Right did not reopen the submenu")
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.Caret() != 4 || state.menuOpen >= 0 {
		t.Fatal("Enter did not navigate and close the menu")
	}

	doc.Editor = text.NewEditor("replacement")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, parent)
	if err := driver.Key(gfx.KeyDown, 0); err != nil {
		t.Fatal(err)
	}
	if driver.Snapshot().Focused != changeHistoryScope.At(2) {
		t.Fatal("unavailable history focused disabled navigation")
	}
}

func TestChangeHistoryCommandsNavigateBlocksAndWrap(t *testing.T) {
	state, _ := openChangeHistoryFixture(t, "one\ntwo\nthree\nfour\n")
	doc := state.space.Current()
	doc.Editor.MoveTo(len([]rune("one\n")), false)
	doc.Editor.Insert("X")
	doc.Editor.MoveTo(len([]rune("oneX\ntwo\nthree\n")), false)
	doc.Editor.Insert("Y")
	doc.Editor.MoveTo(0, false)
	line := func() int {
		return strings.Count(doc.Editor.String()[:doc.Editor.ByteOffset(doc.Editor.Caret())], "\n")
	}
	for _, want := range []int{1, 3, 1} {
		state.command("Next Change")
		if got := line(); got != want {
			t.Fatalf("Next Change line = %d, want %d; notice = %q", got, want, state.notice)
		}
	}
	state.command("Previous Change")
	if got := line(); got != 3 {
		t.Fatalf("Previous Change line = %d, want 3", got)
	}
}

func TestChangeHistoryCommandsPreserveUnchangedSelectionAndDirtyClear(t *testing.T) {
	state, _ := openChangeHistoryFixture(t, "one\ntwo\n")
	doc := state.space.Current()
	doc.Editor.SetSelection(text.Range{Start: 1, End: 3})
	before := doc.Editor.Selection()
	state.command("Next Change")
	if doc.Editor.Selection() != before || state.notice != "No changes to navigate" {
		t.Fatalf("unchanged selection moved or notice wrong: %v, %q", doc.Editor.Selection(), state.notice)
	}
	doc.Editor.MoveTo(4, false)
	doc.Editor.Insert("X")
	value := doc.Editor.String()
	state.command("Clear Change History")
	if doc.Editor.String() != value || !doc.Dirty() || doc.Editor.CanUndo() || state.notice != "Change history cleared" {
		t.Fatalf("clear changed document state: text=%q dirty=%v notice=%q", doc.Editor.String(), doc.Dirty(), state.notice)
	}
	state.command("Next Change")
	if state.notice != "No changes to navigate" {
		t.Fatalf("clear retained change markers: %q", state.notice)
	}
}

func TestClearChangeHistoryCommandRefusesLiveComposition(t *testing.T) {
	state, _ := openChangeHistoryFixture(t, "abc\n")
	doc := state.space.Current()
	if !doc.Editor.EnableTentativeComposition() {
		t.Fatal("tagged composition unavailable")
	}
	doc.Editor.MoveTo(1, false)
	if !doc.Editor.ApplyComposition(text.CompositionEvent{Phase: text.CompositionUpdate, Text: "か"}) {
		t.Fatal("cannot start composition")
	}
	before := doc.Editor.String()
	state.command("Clear Change History")
	if !strings.Contains(state.notice, "composition") || !doc.Editor.CompositionActive() || doc.Editor.String() != before {
		t.Fatalf("refused Clear changed composition: %q, active=%v, notice=%q", doc.Editor.String(), doc.Editor.CompositionActive(), state.notice)
	}
	if !doc.Editor.ApplyComposition(text.CompositionEvent{Phase: text.CompositionCancel}) || doc.Dirty() {
		t.Fatal("refused Clear prevented cancellation or changed the save boundary")
	}
}

func TestChangeHistoryWorkflowSaveUndoBranchClearAndSave(t *testing.T) {
	state, _ := openChangeHistoryFixture(t, "alpha\nbeta\ngamma\n")
	doc := state.space.Current()
	path := doc.Path
	margin := func(wantLine int, wantColor gfx.Color) {
		t.Helper()
		bars := state.changeMarginLines(doc)
		if len(bars) == 0 || bars[len(bars)-1].Line != wantLine || bars[len(bars)-1].Fill != wantColor {
			t.Fatalf("margin = %+v, want line %d color %+v", bars, wantLine, wantColor)
		}
		for _, bar := range bars {
			if bar.Line != wantLine {
				t.Fatalf("margin changed another line: %+v", bars)
			}
		}
	}
	if bars := state.changeMarginLines(doc); len(bars) != 0 {
		t.Fatalf("opened document already has changes: %+v", bars)
	}
	doc.Editor.MoveTo(len([]rune("alpha\n")), false)
	doc.Editor.Insert("X")
	margin(1, palette.changeModified)
	state.command("Save")
	if doc.Dirty() {
		t.Fatalf("Save did not establish a clean boundary: %q", state.notice)
	}
	margin(1, palette.changeSaved)
	state.command("Undo")
	if doc.Editor.String() != "alpha\nbeta\ngamma\n" || !doc.Dirty() {
		t.Fatalf("Undo lost saved boundary: %q dirty=%v", doc.Editor.String(), doc.Dirty())
	}
	margin(1, palette.changeReverted)
	state.command("Redo")
	if doc.Dirty() {
		t.Fatal("Redo did not return to the saved boundary")
	}
	margin(1, palette.changeSaved)
	state.command("Undo")
	doc.Editor.MoveTo(len([]rune("alpha\n")), false)
	doc.Editor.Insert("Y")
	if doc.Editor.CanRedo() {
		t.Fatal("detached branch retained Redo")
	}
	margin(1, palette.changeModified)
	value := doc.Editor.String()
	state.command("Clear Change History")
	if doc.Editor.String() != value || !doc.Dirty() || doc.Editor.AtSavePoint() || doc.Editor.CanUndo() || doc.Editor.CanRedo() {
		t.Fatalf("Clear changed the detached save boundary: %q dirty=%v", doc.Editor.String(), doc.Dirty())
	}
	if bars := state.changeMarginLines(doc); len(bars) != 0 {
		t.Fatalf("Clear retained margin bars: %+v", bars)
	}
	doc.Editor.Insert("Z")
	margin(1, palette.changeModified)
	state.command("Save")
	margin(1, palette.changeSaved)
	if doc.Dirty() {
		t.Fatal("Save after Clear remained dirty")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != doc.Editor.String() {
		t.Fatalf("saved bytes = %q, err=%v; editor=%q", data, err, doc.Editor.String())
	}
}

func TestChangeHistoryWorkflowMacroMultipleAndRectangle(t *testing.T) {
	const initial = "cat\ncat\nlast\n"
	for _, variant := range []struct {
		name, want string
		lines      []int
		edit       func(*testing.T, *model, *app.Driver)
	}{
		{"macro", "Xcat\ncat\nlast\n", []int{0}, func(t *testing.T, state *model, driver *app.Driver) {
			state.command("Record Macro")
			if err := driver.Type("X"); err != nil {
				t.Fatal(err)
			}
			state.command("Stop Recording")
			state.command("Undo")
			state.command("Playback Macro")
		}},
		{"multiple", "X\nX\nlast\n", []int{0, 1}, func(t *testing.T, state *model, driver *app.Driver) {
			state.space.Current().Editor.SetSelection(text.Range{Start: 0, End: 3})
			state.multiSelectAll(document.SearchOptions{})
			state.invalidator.Invalidate()
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
			if err := driver.Type("X"); err != nil {
				t.Fatal(err)
			}
		}},
		{"rectangle", "Xat\nXat\nlast\n", []int{0, 1}, func(t *testing.T, state *model, driver *app.Driver) {
			state.command("Begin/End Select in Column Mode")
			state.space.Current().Editor.MoveTo(5, false)
			state.command("Begin/End Select in Column Mode")
			if err := driver.Type("X"); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(variant.name, func(t *testing.T) {
			state, driver := openChangeHistoryFixture(t, initial)
			doc := state.space.Current()
			state.showInActivePane(doc)
			state.invalidator.Invalidate()
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, editorID)
			doc.Editor.MoveTo(0, false)
			variant.edit(t, state, driver)
			if got := doc.Editor.String(); got != variant.want || !doc.Dirty() {
				t.Fatalf("edit = %q, dirty=%v, notice=%q", got, doc.Dirty(), state.notice)
			}
			if data, err := os.ReadFile(doc.Path); err != nil || string(data) != initial {
				t.Fatalf("edit changed disk before Save: %q, %v", data, err)
			}
			state.command("Save")
			if doc.Dirty() {
				t.Fatalf("Save remained dirty: %q", state.notice)
			}
			bars := state.changeMarginLines(doc)
			if len(bars) != len(variant.lines) {
				t.Fatalf("saved margin = %+v, want lines %v", bars, variant.lines)
			}
			for index, bar := range bars {
				if bar.Line != variant.lines[index] || bar.Fill != palette.changeSaved {
					t.Fatalf("saved margin = %+v, want green lines %v", bars, variant.lines)
				}
			}
			state.command("Undo")
			if doc.Editor.String() != initial || !doc.Dirty() || !doc.Editor.CanRedo() {
				t.Fatalf("one Undo = %q, dirty=%v, redo=%v", doc.Editor.String(), doc.Dirty(), doc.Editor.CanRedo())
			}
			state.command("Redo")
			if doc.Editor.String() != variant.want || doc.Dirty() {
				t.Fatalf("Redo = %q, dirty=%v", doc.Editor.String(), doc.Dirty())
			}
		})
	}
}

func TestChangeHistoryWorkflowReloadRefusalAndFreshSource(t *testing.T) {
	state, driver := openChangeHistoryFixture(t, "one\ntwo\n")
	doc := state.space.Current()
	doc.Editor.MoveTo(4, false)
	doc.Editor.Insert("X")
	oldConsumer, err := doc.ChangeHistory()
	if err != nil {
		t.Fatal(err)
	}
	selection := text.Range{Start: 0, End: 3}
	doc.Editor.SetSelection(selection)
	value := doc.Editor.String()
	state.command("Reload from Disk")
	if state.pendingReload < 0 {
		t.Fatal("dirty reload did not ask for a decision")
	}
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, reloadCancelID)
	if state.space.Current() != doc || doc.Editor.String() != value || doc.Editor.Selection() != selection || !doc.Dirty() {
		t.Fatal("cancelled reload changed the dirty document")
	}
	state.command("Clear Change History")
	if doc.Editor.String() != value || doc.Editor.Selection() != selection || !doc.Dirty() || doc.Editor.AtSavePoint() || doc.Editor.CanUndo() || doc.Editor.CanRedo() {
		t.Fatal("Clear changed selection, text, or the unsaved boundary")
	}
	if current, err := doc.ChangeHistory(); err != nil || current == oldConsumer {
		t.Fatalf("Clear did not establish a fresh generation: %v", err)
	}
	state.command("Save")
	if doc.Dirty() {
		t.Fatalf("Save after Clear failed: %q", state.notice)
	}
	beforeReload, err := doc.ChangeHistory()
	if err != nil {
		t.Fatal(err)
	}
	state.command("Reload from Disk")
	reloaded := state.space.Current()
	newConsumer, err := reloaded.ChangeHistory()
	if err != nil || reloaded == doc || newConsumer == beforeReload || reloaded.Editor.String() != value || reloaded.Dirty() || len(state.changeMarginLines(reloaded)) != 0 {
		t.Fatalf("reload did not bind a clean independent source: %v, notice=%q", err, state.notice)
	}
}

func TestChangeHistoryCurrentDocumentEqualTargetsKeepRealUndo(t *testing.T) {
	const original = "cat\ncat\nlast\n"
	state, driver := openChangeHistoryFixture(t, original)
	doc := state.space.Current()
	state.query.Insert("cat")
	state.replacement.Insert("cat")
	clickID(t, driver, searchID)
	clickID(t, driver, replaceAllID)
	if doc.Editor.String() != original || !doc.Dirty() || !doc.Editor.CanUndo() {
		t.Fatalf("equal target lost its real history: %q dirty=%v undo=%v", doc.Editor.String(), doc.Dirty(), doc.Editor.CanUndo())
	}
	if data, err := os.ReadFile(doc.Path); err != nil || string(data) != original {
		t.Fatalf("equal target changed disk: %q, %v", data, err)
	}
	if bars := state.changeMarginLines(doc); len(bars) != 2 || bars[0].Line != 0 || bars[1].Line != 1 {
		t.Fatalf("equal targets did not produce two changed lines: %+v", bars)
	}
	state.command("Save")
	if doc.Dirty() {
		t.Fatal("Save left equal-target edits dirty")
	}
	savedBars := state.changeMarginLines(doc)
	if len(savedBars) != 2 {
		t.Fatalf("Save lost equal-target editions: %+v", savedBars)
	}
	for _, bar := range savedBars {
		if bar.Fill != palette.changeSaved {
			t.Fatalf("saved equal-target edition = %+v", bar)
		}
	}
	state.command("Undo")
	if doc.Editor.String() != original || !doc.Dirty() || !doc.Editor.CanRedo() {
		t.Fatalf("Undo lost equal-target history: %q dirty=%v redo=%v", doc.Editor.String(), doc.Dirty(), doc.Editor.CanRedo())
	}
	state.command("Redo")
	if doc.Editor.String() != original || doc.Dirty() {
		t.Fatalf("Redo did not restore equal-target save point: %q dirty=%v", doc.Editor.String(), doc.Dirty())
	}
}

func TestChangeHistoryExpiredFramesFailClosedUntilClear(t *testing.T) {
	state, driver := openChangeHistoryFixture(t, "one\ntwo\n")
	doc := state.space.Current()
	for range 300 {
		doc.Editor.SetSavePoint()
	}
	caret := doc.Editor.Caret()
	state.command("Next Change")
	if doc.Editor.Caret() != caret || !strings.Contains(state.notice, "unavailable") {
		t.Fatalf("expired history looked like no changes: caret=%d notice=%q", doc.Editor.Caret(), state.notice)
	}
	state.command("Change History")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	navigationDisabled, clearEnabled := false, false
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == changeHistoryScope.At(0) {
			navigationDisabled = hit.Disabled
		}
		if hit.ID == changeHistoryScope.At(2) {
			clearEnabled = !hit.Disabled
		}
	}
	if !navigationDisabled || !clearEnabled {
		t.Fatal("expired history did not disable navigation while allowing a reset")
	}
	clickID(t, driver, changeHistoryScope.At(2))
	if state.notice != "Change history cleared" || doc.Dirty() {
		t.Fatalf("Clear did not restore a clean available generation: %q dirty=%v", state.notice, doc.Dirty())
	}
	doc.Editor.MoveTo(4, false)
	doc.Editor.Insert("X")
	if _, err := doc.ChangeHistory(); err != nil || len(state.changeMarginLines(doc)) == 0 {
		t.Fatalf("new generation did not track later edits: %v", err)
	}
}
