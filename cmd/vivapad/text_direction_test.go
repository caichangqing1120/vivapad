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
	"vivapad/internal/session"
)

func TestTextDirectionCommandsArePerDocumentAndPreserveEditing(t *testing.T) {
	state, _ := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("\u05d0\u05d1 mixed")
	doc.Editor.MoveTo(6, false)
	doc.Editor.MoveTo(2, true)
	selection, caret := doc.Editor.Selection(), doc.Editor.Caret()
	value, undo, wrap := doc.Editor.String(), doc.Editor.CanUndo(), state.wrap
	if !slices.Contains(menuCommands[3][:], "Text Direction RTL") || !slices.Contains(menuCommands[3][:], "Text Direction LTR") {
		t.Fatal("View menu is missing text direction commands")
	}
	shortcut := newConfig(state).OnShortcut
	if result := shortcut(gfx.KeyEvent{Text: "r", Target: editorID, Modifiers: gfx.ModifierControl | gfx.ModifierAlt}); !result.Handled || !doc.RTL {
		t.Fatalf("RTL shortcut did not update the active buffer: %+v", result)
	}
	if doc.Editor.String() != value || doc.Editor.Selection() != selection || doc.Editor.Caret() != caret || doc.Editor.CanUndo() != undo || state.wrap != wrap {
		t.Fatal("switching direction changed editing or wrap state")
	}
	state.command("New")
	if other := state.space.Current(); other == doc || other.RTL || !doc.RTL {
		t.Fatal("direction leaked into a different tab")
	}
	state.command("Text Direction RTL")
	if result := shortcut(gfx.KeyEvent{Text: "l", Target: editorID, Modifiers: gfx.ModifierControl | gfx.ModifierAlt}); !result.Handled || state.space.Current().RTL || !doc.RTL {
		t.Fatal("LTR shortcut affected a different buffer or did not reset direction")
	}
}

func TestTextDirectionPersistsClonedDocument(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "text.txt")
	if err := os.WriteFile(path, []byte("sample"), 0600); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "state", "session.json")
	state := newStartupModel([]string{path}, file)
	doc := state.space.Current()
	state.command("Clone to Other View")
	if !state.splitEnabled || state.paneDocument(0) != doc || state.paneDocument(1) != doc {
		t.Fatal("Clone to Other View did not share the document")
	}
	state.command(textDirectionRTL)
	state.command("Focus on Another View")
	state.command(textDirectionLTR)
	if doc.RTL {
		t.Fatal("changing direction from the other pane left the shared document RTL")
	}
	state.command(textDirectionRTL)
	closed := false
	newConfig(state).OnCloseRequest(func() { closed = true })
	if !closed {
		t.Fatalf("did not close and save the clean workspace: %q", state.notice)
	}
	restored := newStartupModel(nil, file)
	left, right := restored.paneDocument(0), restored.paneDocument(1)
	if !restored.splitEnabled || left != right || left.Path != path || !left.RTL {
		t.Fatalf("private session lost the shared RTL document: split=%v, shared=%v, path=%q, RTL=%v", restored.splitEnabled, left == right, left.Path, left.RTL)
	}
}

func TestTextDirectionRoundTripsNPPCloneAndRejectsConflict(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "shared.txt")
	if err := os.WriteFile(path, []byte("saved"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	state.command("Clone to Other View")
	state.command(textDirectionRTL)
	file := filepath.Join(root, "shared.xml")
	state.saveNPPSessionSelection(app.SaveFileSelection{Path: file})
	stored, err := session.LoadNPP(file)
	if err != nil || len(stored.Main) != 1 || len(stored.Sub) != 1 || !stored.Main[0].RTL || !stored.Sub[0].RTL {
		t.Fatalf("NPP export lost the RTL clone: %+v, %v (notice %q)", stored, err, state.notice)
	}
	for _, conflicting := range []bool{false, true} {
		if conflicting {
			stored.Sub[0].RTL = false
			file = filepath.Join(root, "conflicting.xml")
			if err := session.SaveNPP(file, stored); err != nil {
				t.Fatal(err)
			}
		}
		loaded, driver := startDriver(t)
		original := loaded.space.Current()
		if err := driver.QueueOpenFiles([]string{file}, nil); err != nil {
			t.Fatal(err)
		}
		clickID(t, driver, menuScope.At(0))
		clickID(t, driver, menuItemScope.At(13))
		clickID(t, driver, sessionScope.At(0))
		if conflicting {
			if loaded.splitEnabled || loaded.space.Current() != original || len(loaded.space.Documents) != 1 || !strings.Contains(loaded.notice, "conflicting document state") {
				t.Fatalf("conflicting RTL clone changed the workspace: %q", loaded.notice)
			}
			continue
		}
		left, right := loaded.paneDocument(0), loaded.paneDocument(1)
		if !loaded.splitEnabled || left != right || !left.RTL {
			t.Fatalf("NPP import lost the shared RTL document: %q", loaded.notice)
		}
	}
}

func TestTextDirectionMirrorsDocumentMapPreview(t *testing.T) {
	state, driver := startDriver(t)
	state.space.Current().Editor.Insert("   first preview line\nsecond line")
	state.command("Document Map")
	preview := func() (gfx.Rect, []gfx.Rect) {
		t.Helper()
		state.invalidator.Invalidate()
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		bounds := documentMapHit(t, driver)
		var bars []gfx.Rect
		for _, primitive := range driver.Snapshot().Primitives {
			if primitive.Color == gfx.RGB(158, 165, 172) && primitive.Bounds.X >= bounds.X && primitive.Bounds.X+primitive.Bounds.Width <= bounds.X+bounds.Width {
				bars = append(bars, primitive.Bounds)
			}
		}
		return bounds, bars
	}
	leftBounds, ltr := preview()
	state.command(textDirectionRTL)
	rightBounds, rtl := preview()
	if leftBounds != rightBounds || len(ltr) < 2 || len(rtl) != len(ltr) {
		t.Fatalf("preview lost the map or its lines: %v %v -> %v %v", leftBounds, ltr, rightBounds, rtl)
	}
	for i, bar := range ltr {
		want := gfx.Rect{X: 2*leftBounds.X + leftBounds.Width - bar.X - bar.Width, Y: bar.Y, Width: bar.Width, Height: bar.Height}
		if delta := rtl[i].X - want.X; delta < -.001 || delta > .001 || rtl[i].Y != want.Y || rtl[i].Width != want.Width || rtl[i].Height != want.Height {
			t.Fatalf("RTL preview line %d = %+v, want %+v", i, rtl[i], want)
		}
	}
}

func TestTextDirectionCollapsesMixedSelectionVisually(t *testing.T) {
	engine := app.PlatformTextEngine()
	if _, ok := engine.(gfx.FittedMonoCaretEngine); !ok {
		t.Skip("platform fitted text engine unavailable")
	}
	for _, mode := range []struct {
		name, value string
		rtl         bool
		start, end  int
		left, right int
	}{
		{"LTR Hebrew run", "a\u05d0\u05d1z", false, 1, 2, 2, 1},
		{"RTL Latin run", "\u05d0\u05d1 abc", true, 4, 5, 4, 5},
	} {
		t.Run(mode.name, func(t *testing.T) {
			state := newModel()
			doc := state.space.Current()
			doc.Editor.Insert(mode.value)
			if mode.rtl {
				state.command(textDirectionRTL)
			}
			var canvas gfx.Canvas
			canvas.SetTextEngine(engine)
			driver, err := app.NewDriver(newConfig(state), &canvas)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(driver.Close)
			clickID(t, driver, editorID)
			var positions []float32
			for _, edge := range []struct {
				key  gfx.Key
				want int
			}{{gfx.KeyLeft, mode.left}, {gfx.KeyRight, mode.right}} {
				doc.Editor.SetSelection(text.Range{Start: mode.start, End: mode.end})
				state.invalidator.Invalidate()
				if err := driver.Key(edge.key, 0); err != nil {
					t.Fatal(err)
				}
				if got := doc.Editor.Selection(); got != (text.Range{Start: edge.want, End: edge.want}) {
					t.Fatalf("key %v collapsed to %+v, want %d", edge.key, got, edge.want)
				}
				if bounds, shown := state.codeEditor().CaretBounds(); shown {
					positions = append(positions, bounds.X)
				}
			}
			if len(positions) != 2 || positions[0] >= positions[1] {
				t.Fatalf("visual edges in paint order: %v", positions)
			}
		})
	}
}

func TestTextDirectionKeepsLTRMixedMacroRecording(t *testing.T) {
	engine := app.PlatformTextEngine()
	if _, ok := engine.(gfx.FittedMonoCaretEngine); !ok {
		t.Skip("platform fitted text engine unavailable")
	}
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("a\u05d0\u05d1z")
	doc.Editor.MoveTo(2, false)
	var canvas gfx.Canvas
	canvas.SetTextEngine(engine)
	driver, err := app.NewDriver(newConfig(state), &canvas)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	clickID(t, driver, editorID)
	doc.Editor.MoveTo(2, false)
	state.invalidator.Invalidate()
	state.startMacro()
	if err := driver.Key(gfx.KeyLeft, 0); err != nil {
		t.Fatal(err)
	}
	if !state.macroRecording || len(state.macroSteps) != 1 || state.macroSteps[0].Command != app.TextMoveLeft {
		t.Fatalf("LTR arrow interrupted or lost the macro step: %q, %+v", state.notice, state.macroSteps)
	}
}

func TestTextDirectionCompactMenuAndNativeTextCommand(t *testing.T) {
	engine := app.PlatformTextEngine()
	if _, ok := engine.(gfx.FittedMonoCaretEngine); !ok {
		t.Skip("platform fitted text engine unavailable")
	}
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("\u05d0\u05d1\u05d2")
	doc.Editor.SetSelection(text.Range{})
	var canvas gfx.Canvas
	canvas.SetTextEngine(engine)
	driver, err := app.NewDriver(newConfig(state), &canvas)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(slices.Index(menuCommands[3][:], textDirectionRTL)))
	if !doc.RTL {
		t.Fatal("compact View menu did not set RTL")
	}
	editor := state.codeEditor()
	before, ok := editor.CaretBounds()
	if !ok || before.X < 320 {
		t.Fatalf("RTL caret did not start at the right edge: %+v", before)
	}
	if err := driver.Key(gfx.KeyLeft, gfx.ModifierShift); err != nil {
		t.Fatal(err)
	}
	after, ok := editor.CaretBounds()
	if !ok || after.X >= before.X || doc.Editor.Selection() != (text.Range{Start: 0, End: 1}) {
		t.Fatalf("Shift+Left did not extend to the next visual character: x %g -> %g, selection %+v", before.X, after.X, doc.Editor.Selection())
	}
}
