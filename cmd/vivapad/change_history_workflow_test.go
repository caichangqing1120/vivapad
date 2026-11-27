package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

// H06 exit criterion: the whole change-history workflow, driven through the real
// commands, keeps the margin, the save boundary and Dirty consistent.

func workflowKinds(state *model) []gfx.Color {
	lines := state.changeMarginLines(state.space.Current())
	var kinds []gfx.Color
	last := -1
	for _, line := range lines {
		if line.Line != last {
			kinds = append(kinds, line.Fill)
			last = line.Line
		} else {
			kinds[len(kinds)-1] = line.Fill
		}
	}
	return kinds
}

func TestChangeHistoryWorkflowTypingSaveUndoClear(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workflow.txt")
	if err := os.WriteFile(path, []byte("alpha\nbeta\ngamma\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	state, _ := startDriver(t)
	if err := state.space.Open(path); err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	doc := state.space.Current()
	if kinds := workflowKinds(state); len(kinds) != 0 {
		t.Fatalf("a freshly opened document paints %v", kinds)
	}

	// 1. Typing marks the edited line as modified.
	doc.Editor.MoveTo(len([]rune("alpha\n")), false)
	doc.Editor.Insert("X")
	if kinds := workflowKinds(state); len(kinds) != 1 || kinds[0] != classicPalette.changeModified {
		t.Fatalf("after typing kinds = %v, want one modified line", kinds)
	}
	if !doc.Dirty() {
		t.Fatal("typing must make the document dirty")
	}

	// 2. Save turns the same line into a saved change and clears Dirty.
	state.command("Save")
	if doc.Dirty() {
		t.Fatalf("the Save command left the document dirty: %q", state.notice)
	}
	if !doc.Editor.AtSavePoint() {
		t.Fatal("the Save command did not move the editor save point")
	}
	if kinds := workflowKinds(state); len(kinds) != 1 || kinds[0] != classicPalette.changeSaved {
		t.Fatalf("after save kinds = %v, want one saved line", kinds)
	}
	if doc.Dirty() {
		t.Fatal("save must clear Dirty")
	}

	// 3. Undo returns the bytes but the history keeps the change: the line carries
	// a reverted/deletion marker and the document is modified again because the
	// saved boundary now lives in Redo. Redo restores the saved state.
	state.command("Undo")
	if doc.Editor.String() != "alpha\nbeta\ngamma\n" {
		t.Fatalf("Undo did not restore the loaded bytes: %q", doc.Editor.String())
	}
	lines := state.changeMarginLines(doc)
	if len(lines) != 1 || lines[0].Line != 1 || lines[0].Fill != classicPalette.changeReverted {
		t.Fatalf("after undo decorations = %+v, want the reverted deletion marker", lines)
	}
	if !doc.Dirty() {
		t.Fatal("after undo the saved boundary lives in Redo, so the document is modified")
	}
	state.command("Redo")
	if kinds := workflowKinds(state); len(kinds) != 1 || kinds[0] != classicPalette.changeSaved {
		t.Fatalf("after redo kinds = %v, want the saved line again", kinds)
	}
	if doc.Dirty() {
		t.Fatal("Redo returns to the saved state")
	}

	// 4. A detached branch: undo and type something else.
	doc.Editor.Undo()
	doc.Editor.MoveTo(len([]rune("alpha\n")), false)
	doc.Editor.Insert("Y")
	if kinds := workflowKinds(state); len(kinds) != 1 || kinds[0] != classicPalette.changeModified {
		t.Fatalf("detached branch kinds = %v, want one modified line", kinds)
	}
	if doc.Editor.CanRedo() {
		t.Fatal("the branch must discard Redo")
	}

	// 5. Clear forgets the history but keeps the text, Dirty and the save boundary.
	text := doc.Editor.String()
	state.command("Clear Change History")
	if state.notice != "Change history cleared" {
		t.Fatalf("clear notice = %q", state.notice)
	}
	if kinds := workflowKinds(state); len(kinds) != 0 {
		t.Fatalf("after clear kinds = %v, want none", kinds)
	}
	if doc.Editor.String() != text || !doc.Dirty() {
		t.Fatalf("clear changed text %q or laundered Dirty %v", doc.Editor.String(), doc.Dirty())
	}
	// Dirty() also compares the text, so assert the boundary itself: the reset must
	// leave the save point unreachable rather than rebasing onto the new history.
	if doc.Editor.AtSavePoint() {
		t.Fatal("clear rebased the save point onto the modified buffer")
	}
	if doc.Editor.CanUndo() || doc.Editor.CanRedo() {
		t.Fatal("clear kept history units")
	}

	// The decorations reach the editor snapshot through the application hookup, not
	// only through the supplier this test calls directly.
	snapshot := components.CodeEditorSnapshot{}
	state.changeHistoryView = "both"
	state.applyChangeDecorations(&snapshot, doc)
	decorations := actualChangeDecorations(t, snapshot)
	if len(decorations.GutterBars) != 0 || decorations.GutterWidth != 9 {
		t.Fatalf("clear must keep the empty reserved margin: %+v", decorations)
	}
	for _, layer := range decorations.IndicatorLayers {
		if len(layer.Spans) != 0 {
			t.Fatalf("clear kept indicator spans: %+v", layer)
		}
	}

	// 6. A later edit and Save work against the new generation.
	doc.Editor.MoveTo(len([]rune("alpha\nYbeta\n")), false)
	doc.Editor.Insert("Z")
	if kinds := workflowKinds(state); len(kinds) != 1 || kinds[0] != classicPalette.changeModified {
		t.Fatalf("post-clear edit kinds = %v", kinds)
	}
	state.command("Save")
	if doc.Dirty() {
		t.Fatalf("the Save command after clear left the document dirty: %q", state.notice)
	}
	if kinds := workflowKinds(state); len(kinds) != 1 || kinds[0] != classicPalette.changeSaved {
		t.Fatalf("post-clear save kinds = %v", kinds)
	}
	if doc.Dirty() {
		t.Fatal("the post-clear save must clear Dirty")
	}
	// The saved bytes follow the document, so a reload sees the same text.
	reloaded, err := reopenDocument(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if reloaded != doc.Editor.String() {
		t.Fatalf("saved bytes = %q, want %q", reloaded, doc.Editor.String())
	}
}

func reopenDocument(path string) (string, error) {
	data, err := os.ReadFile(path)
	return string(data), err
}

func TestChangeHistoryWorkflowEditingVariants(t *testing.T) {
	const initial = "cat\ncat\nlast\n"
	for _, variant := range []struct {
		name, want string
		lineCount  int
		edit       func(*testing.T, *model, *app.Driver)
	}{
		{"macro", "Xcat\ncat\nlast\n", 1, func(t *testing.T, state *model, driver *app.Driver) {
			state.command("Record Macro")
			if err := driver.Type("X"); err != nil {
				t.Fatal(err)
			}
			state.command("Stop Recording")
			state.command("Undo")
			state.command("Playback Macro")
		}},
		{"multiple", "X\nX\nlast\n", 2, func(t *testing.T, state *model, driver *app.Driver) {
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
		{"rectangle", "Xat\nXat\nlast\n", 2, func(t *testing.T, state *model, driver *app.Driver) {
			state.command("Begin/End Select in Column Mode")
			state.space.Current().Editor.MoveTo(5, false)
			state.command("Begin/End Select in Column Mode")
			if err := driver.Type("X"); err != nil {
				t.Fatal(err)
			}
		}},
		{"equal-targets", initial, 2, func(t *testing.T, state *model, driver *app.Driver) {
			state.query.Insert("cat")
			state.replacement.Insert("cat")
			clickID(t, driver, searchID)
			clickID(t, driver, replaceAllID)
		}},
	} {
		t.Run(variant.name, func(t *testing.T) {
			state, driver := startDriver(t)
			path := filepath.Join(t.TempDir(), "workflow.txt")
			if err := os.WriteFile(path, []byte(initial), 0600); err != nil {
				t.Fatal(err)
			}
			if err := state.space.Open(path); err != nil {
				t.Fatal(err)
			}
			doc := state.space.Current()
			state.showInActivePane(doc)
			state.invalidator.Invalidate()
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, editorID)
			doc.Editor.MoveTo(0, false)
			variant.edit(t, state, driver)
			if doc.Editor.String() != variant.want || !doc.Dirty() || len(workflowKinds(state)) == 0 {
				t.Fatalf("edit = %q, dirty=%v, changes=%v, notice=%q", doc.Editor.String(), doc.Dirty(), workflowKinds(state), state.notice)
			}
			if disk, err := os.ReadFile(path); err != nil || string(disk) != initial {
				t.Fatalf("edit changed disk before Save: %q / %v", disk, err)
			}
			state.command("Save")
			if doc.Dirty() || !doc.Editor.AtSavePoint() {
				t.Fatalf("Save did not establish the saved boundary: %q", state.notice)
			}
			savedLines := append([]components.CodeEditorGutterBar(nil), state.changeMarginLines(doc)...)
			if len(savedLines) != variant.lineCount {
				t.Fatalf("Save decorations = %+v, want %d saved lines", savedLines, variant.lineCount)
			}
			for index, line := range savedLines {
				if line.Line != index || line.Fill != classicPalette.changeSaved {
					t.Fatalf("Save decoration %d = %+v, want saved line %d", index, line, index)
				}
			}
			state.command("Undo")
			if doc.Editor.String() != initial || !doc.Dirty() || !doc.Editor.CanRedo() {
				t.Fatalf("Undo = %q, dirty=%v, redo=%v", doc.Editor.String(), doc.Dirty(), doc.Editor.CanRedo())
			}
			if lines := state.changeMarginLines(doc); len(lines) == 0 || reflect.DeepEqual(lines, savedLines) {
				t.Fatalf("Undo lost reversion decorations or kept the saved state: %+v", lines)
			}
			state.command("Redo")
			if doc.Editor.String() != variant.want || doc.Dirty() {
				t.Fatalf("Redo = %q, dirty=%v", doc.Editor.String(), doc.Dirty())
			}
			if lines := state.changeMarginLines(doc); !reflect.DeepEqual(lines, savedLines) {
				t.Fatalf("Redo decorations = %+v, want saved decorations %+v", lines, savedLines)
			}
			state.command("Undo")
			clickID(t, driver, editorID)
			if err := driver.Type("Z"); err != nil {
				t.Fatal(err)
			}
			if doc.Editor.CanRedo() || !doc.Dirty() {
				t.Fatal("detached edit kept Redo or the saved boundary")
			}
			before := doc.Editor.String()
			state.command("Reload from Disk")
			if state.pendingReload < 0 {
				t.Fatal("dirty reload did not require a decision")
			}
			state.invalidator.Invalidate()
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, reloadCancelID)
			if state.space.Current() != doc || doc.Editor.String() != before || !doc.Dirty() {
				t.Fatal("cancelled reload changed the dirty document")
			}
			selection := doc.Editor.Selection()
			state.command("Change History")
			state.invalidator.Invalidate()
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, changeHistoryScope.At(2))
			if doc.Editor.String() != before || doc.Editor.Selection() != selection || !doc.Dirty() || doc.Editor.AtSavePoint() || doc.Editor.CanUndo() || doc.Editor.CanRedo() || len(workflowKinds(state)) != 0 {
				t.Fatal("Clear changed the buffer/selection/save boundary or retained history")
			}
			state.command("Save")
			oldConsumer, err := doc.ChangeHistory()
			if err != nil {
				t.Fatal(err)
			}
			state.command("Reload from Disk")
			reloaded := state.space.Current()
			freshConsumer, err := reloaded.ChangeHistory()
			if err != nil || reloaded == doc || freshConsumer == oldConsumer || reloaded.Editor.String() != before || reloaded.Dirty() || len(workflowKinds(state)) != 0 {
				t.Fatalf("reload did not establish a clean independent source: %v, %q", err, state.notice)
			}
		})
	}
}
