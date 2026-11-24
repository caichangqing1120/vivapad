package main

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestPostItKeepsBothEditorsAndRestoresTheWindow(t *testing.T) {
	state, driver := startDriver(t)
	left := state.space.Current()
	left.Editor.Insert("left draft")
	right := state.space.NewDocument()
	right.Editor.Insert("right draft")
	right.Editor.SetSelection(text.Range{Start: 6, End: 11})
	state.toggleSplit()
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	var levels, decorations []bool
	var complete func(error)
	state.requestAlwaysOnTop = func(enabled bool) error { levels = append(levels, enabled); return nil }
	state.requestDecoration = func(decorated bool, done func(error)) error {
		decorations, complete = append(decorations, decorated), done
		return nil
	}
	index := slices.Index(menuCommands[3], postItCommand)
	if index < 0 {
		t.Fatal("View menu does not offer Post-It")
	}
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(index))
	if state.menuSelected(postItCommand) || complete == nil || !slices.Equal(decorations, []bool{false}) {
		t.Fatal("Post-It became active before the native transition finished")
	}
	complete(nil)
	requireEditorCommandFocus(t, driver, rightEditorID)
	hits := driver.Snapshot().Hits
	visible := map[gfx.ID]bool{}
	for _, hit := range hits {
		visible[hit.ID] = true
		if hit.ID == postItRestoreID && (hit.Bounds.X < 0 || hit.Bounds.X+hit.Bounds.Width > 640 || hit.Bounds.Y+hit.Bounds.Height > 400) {
			t.Fatalf("restore button is outside the viewport: %v", hit.Bounds)
		}
	}
	if !state.menuSelected(postItCommand) || !visible[postItRestoreID] || !visible[editorID] || !visible[rightEditorID] ||
		visible[menuScope.At(3)] || visible[saveID] {
		t.Fatal("Post-It did not keep both editors and hide the normal chrome")
	}
	clickID(t, driver, postItRestoreID)
	if !state.menuSelected(postItCommand) || !slices.Equal(decorations, []bool{false, true}) {
		t.Fatal("Post-It hid its restore button before native confirmation")
	}
	complete(nil)
	requireEditorCommandFocus(t, driver, rightEditorID)
	if state.menuSelected(postItCommand) || state.fullscreen || !slices.Equal(levels, []bool{true, false}) ||
		!state.splitEnabled || state.paneDocument(0) != left || state.paneDocument(1) != right || state.activePane != 1 {
		t.Fatalf("workspace did not restore; level requests=%v", levels)
	}
	if err := driver.Type("edit"); err != nil || right.Editor.String() != "right edit" || left.Editor.String() != "left draft" {
		t.Fatalf("first edit did not replace the original right selection: %q, %q, %v", left.Editor.String(), right.Editor.String(), err)
	}
	state.command("Undo")
	if right.Editor.String() != "right draft" || left.Editor.String() != "left draft" {
		t.Fatal("Undo did not restore the original split documents")
	}
}

func TestPostItF12InFullscreenKeepsExistingTopmost(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.SetFullscreen(true); err != nil {
		t.Fatal(err)
	}
	state.alwaysOnTop = true
	state.requestAlwaysOnTop = func(bool) error { t.Error("changed an existing topmost level"); return nil }
	state.requestDecoration = func(bool, func(error)) error { t.Error("changed native decoration in fullscreen"); return nil }
	if err := driver.Key(gfx.KeyF12, 0); err != nil || !state.menuSelected(postItCommand) {
		t.Fatalf("F12 did not enter Post-It in fullscreen: %v", err)
	}
	clickID(t, driver, postItRestoreID)
	if !state.fullscreen || !driver.Fullscreen() || state.menuSelected(postItCommand) || !state.alwaysOnTop {
		t.Fatal("Post-It exit changed the pre-existing fullscreen or topmost mode")
	}
}

func TestPostItF12RefusesUnsupportedTopmostWithoutChangingEditors(t *testing.T) {
	state, driver := startDriver(t)
	left := state.space.Current()
	left.Editor.Insert("left draft")
	right := state.space.NewDocument()
	right.Editor.Insert("right draft")
	selection := text.Range{Start: 6, End: 11}
	right.Editor.SetSelection(selection)
	state.toggleSplit()
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	state.requestAlwaysOnTop = func(bool) error { return app.ErrUnsupported }
	state.requestDecoration = func(bool, func(error)) error {
		t.Error("attempted to change native decorations without topmost support")
		return nil
	}
	frame := driver.Frame()
	if err := driver.Key(gfx.KeyF12, 0); err != nil {
		t.Fatal(err)
	}
	requireEditorCommandFocus(t, driver, rightEditorID)
	if state.menuSelected(postItCommand) || state.postIt.pending || state.alwaysOnTop || driver.Frame() != frame ||
		!strings.Contains(state.notice, app.ErrUnsupported.Error()) || right.Editor.Selection() != selection {
		t.Fatalf("unsupported Post-It changed the window or selection: frame=%v notice=%q selection=%v", driver.Frame(), state.notice, right.Editor.Selection())
	}
	if err := driver.Type("x"); err != nil || left.Editor.String() != "left draft" || right.Editor.String() != "right x" {
		t.Fatalf("first key after refusal: left=%q right=%q error=%v", left.Editor.String(), right.Editor.String(), err)
	}
}

func TestPostItRefusalPreservesLiveComposition(t *testing.T) {
	for _, scenario := range []string{"unsupported-topmost", "rejected-decoration"} {
		t.Run(scenario, func(t *testing.T) {
			state := newModel()
			doc := state.space.Current()
			doc.Editor.Insert("abc")
			doc.Editor.ClearHistory()
			doc.Editor.MoveTo(1, false)
			input := tentativeTextInput(t, state, 0)
			sendComposition(t, input, text.CompositionUpdate, "x")
			assertLiveComposition(t, doc)
			selection, canUndo := doc.Editor.Selection(), doc.Editor.CanUndo()
			before, committed := doc.Editor.String(), doc.Editor.CommittedString()
			var complete func(error)
			state.requestAlwaysOnTop = func(bool) error {
				if scenario == "unsupported-topmost" {
					return app.ErrUnsupported
				}
				return nil
			}
			state.requestDecoration = func(_ bool, done func(error)) error {
				complete = done
				return nil
			}
			state.command(postItCommand)
			if scenario == "rejected-decoration" {
				if complete == nil || !state.postIt.pending || !doc.Editor.CompositionActive() {
					t.Fatal("submitting Post-It disturbed the live composition")
				}
				complete(errors.New("window manager refused"))
			} else if complete != nil {
				t.Fatal("requested decoration after topmost was refused")
			}
			if !doc.Editor.CompositionActive() || doc.Editor.String() != before || doc.Editor.CommittedString() != committed ||
				doc.Editor.Selection() != selection || doc.Editor.CanUndo() != canUndo || state.postIt.active || state.postIt.pending {
				t.Fatalf("refusal disturbed the live composition: text=%q committed=%q selection=%v canUndo=%t notice=%q",
					doc.Editor.String(), doc.Editor.CommittedString(), doc.Editor.Selection(), doc.Editor.CanUndo(), state.notice)
			}
			sendComposition(t, input, text.CompositionCommit, "")
			if doc.Editor.String() != "axbc" || !doc.Editor.Undo() || doc.Editor.String() != "abc" || doc.Editor.CanUndo() {
				t.Fatalf("completing input after refusal did not produce one Undo unit: text=%q canUndo=%t", doc.Editor.String(), doc.Editor.CanUndo())
			}
		})
	}
}

func TestPostItRollsBackFailedNativeRequests(t *testing.T) {
	state, _ := startDriver(t)
	var levels []bool
	rejected := errors.New("frame request refused")
	state.requestAlwaysOnTop = func(enabled bool) error { levels = append(levels, enabled); return nil }
	state.requestDecoration = func(_ bool, done func(error)) error { done(rejected); return nil }
	state.command(postItCommand)
	if state.menuSelected(postItCommand) || !slices.Equal(levels, []bool{true, false}) || !strings.Contains(state.notice, rejected.Error()) {
		t.Fatalf("failed entry left the interface changed: level=%v, notice=%q", levels, state.notice)
	}
	state.requestDecoration = func(decorated bool, done func(error)) error {
		if decorated {
			return rejected
		}
		done(nil)
		return nil
	}
	state.command(postItCommand)
	state.command(postItCommand)
	if !state.menuSelected(postItCommand) || !slices.Equal(levels, []bool{true, false, true, false, true}) {
		t.Fatalf("failed restore did not retain Post-It: level=%v, notice=%q", levels, state.notice)
	}
	state.requestDecoration = func(_ bool, done func(error)) error { done(nil); return nil }
	state.command(postItCommand)
	if state.menuSelected(postItCommand) || levels[len(levels)-1] {
		t.Fatal("retry did not restore the window")
	}
}
