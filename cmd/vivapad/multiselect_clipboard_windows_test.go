package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/hashops"
	"vivapad/tools/windows"
)

func TestSelectionHashWritesWindowsClipboard(t *testing.T) {
	if os.Getenv("VIVAPAD_WINDOWS_CLIPBOARD_TEST") != "1" {
		t.Skip("requires an isolated Windows desktop clipboard")
	}
	path := filepath.Join(t.TempDir(), "mixed-selection-hash.txt")
	if err := os.WriteFile(path, []byte(windows.SelectionHashSource), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModel()
	if err := state.space.Open(path); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	const original = "caf\u00e9 caf\u00e9\nxy\nZ\n"
	if got := doc.Editor.String(); got != original {
		t.Fatalf("decoded buffer = %q, want %q", got, original)
	}
	state.hashAlgorithm = hashops.SHA256
	var canvas gfx.Canvas
	var checked bool
	var checkErr error
	err := app.RunApplication(func(application *app.Application) error {
		window, err := application.OpenWindow(newNativeConfig(state), &canvas)
		if err != nil {
			return err
		}
		application.Post(func() {
			defer window.Close()
			const command = "Generate from selection into clipboard"
			whole := text.Range{Start: 0, End: doc.Editor.RuneCount()}
			doc.Editor.SetSelection(whole)
			sequence := windows.ClipboardSequence()
			state.command(command)
			if windows.ClipboardSequence() == sequence || doc.Editor.Selection() != whole {
				checkErr = fmt.Errorf("single range did not preserve selection and publish digest: %s", state.notice)
				return
			}
			if checkErr = windows.CheckSelectionHashClipboard(); checkErr != nil {
				return
			}
			ranges := []text.Range{{Start: 0, End: 4}, {Start: 5, End: 9}}
			doc.Editor.SetSelection(ranges[1])
			state.multiSelections[0] = multiSelectionState{doc: doc, value: original, ranges: ranges, active: true}
			sequence = windows.ClipboardSequence()
			state.command(command)
			if windows.ClipboardSequence() != sequence || !state.hasMultipleStreamSelections(doc) || doc.Editor.String() != original || doc.Dirty() || doc.Editor.CanUndo() {
				checkErr = fmt.Errorf("multiple ranges changed clipboard or document state: %s", state.notice)
				return
			}
			checkErr = windows.CheckSelectionHashClipboard()
			checked = true
		})
		return nil
	})
	if err != nil || checkErr != nil || !checked {
		t.Fatalf("Windows selection hash: app=%v, checks=%v, completed=%v", err, checkErr, checked)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != windows.SelectionHashSource {
		t.Fatalf("source after hash = %x, read error %v", got, err)
	}
}

func TestMultiSelectionWritesWindowsClipboard(t *testing.T) {
	if os.Getenv("VIVAPAD_WINDOWS_CLIPBOARD_TEST") != "1" {
		t.Skip("requires an isolated Windows desktop clipboard")
	}
	path := filepath.Join(t.TempDir(), "selection.txt")
	if err := os.WriteFile(path, []byte(windows.SelectionClipboardSource), 0600); err != nil {
		t.Fatal(err)
	}
	state := openMultiSelectionClipboardFixture(t, path)
	doc := state.space.Current()
	var canvas gfx.Canvas
	var checkErr error
	err := app.RunApplication(func(application *app.Application) error {
		window, err := application.OpenWindow(newNativeConfig(state), &canvas)
		if err != nil {
			return err
		}
		application.Post(func() {
			defer window.Close()
			for _, command := range []string{"Copy", "Cut"} {
				sequence := windows.ClipboardSequence()
				state.command(command)
				if sequence == windows.ClipboardSequence() {
					checkErr = fmt.Errorf("%s did not write to the system clipboard (notice: %q)", command, state.notice)
					return
				}
				if err := windows.CheckSelectionClipboard(); err != nil {
					checkErr = fmt.Errorf("%s: %w", command, err)
					return
				}
			}
		})
		return nil
	})
	if err != nil || checkErr != nil {
		t.Fatalf("native application: %v; clipboard: %v", err, checkErr)
	}
	if doc.Editor.String() != "b\n" {
		t.Fatalf("Cut left %q", doc.Editor.String())
	}
}

func TestServeWindowsMultiSelectionClipboardFixture(t *testing.T) {
	path := os.Getenv("VIVAPAD_MULTI_SELECTION_FIXTURE")
	if path == "" {
		t.Skip("launched only by the isolated Windows clipboard comparison")
	}
	state := openMultiSelectionClipboardFixture(t, path)
	state.requestEditorCommandFocus()
	config := newNativeConfig(state)
	config.Title = "VivaPad clipboard fixture"
	resolve := config.TextInput
	var ready bool
	var readyErr error
	config.TextInput = func(context app.TextInputContext) *app.TextInput {
		input := resolve(context)
		if !ready && context.ID == editorID && input != nil && input.CopyOverride && input.CopyRectangular {
			ready = true
			readyErr = os.WriteFile(path+".ready", []byte("ready"), 0600)
		}
		return input
	}
	var canvas gfx.Canvas
	if err := app.RunApplication(func(application *app.Application) error {
		window, err := application.OpenWindow(config, &canvas)
		if err == nil {
			state.closeWindow = func() { config.OnCloseRequest(window.Close) }
		}
		return err
	}); err != nil || readyErr != nil || !ready {
		t.Fatalf("native fixture: %v; ready=%v, signal error=%v", err, ready, readyErr)
	}
}

func openMultiSelectionClipboardFixture(t *testing.T, path string) *model {
	t.Helper()
	state := newModel()
	if err := state.space.Open(path); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	if doc.Editor.String() != "a\nb\nc\nd" {
		t.Fatalf("unexpected clipboard fixture: %q", doc.Editor.String())
	}
	ranges := []text.Range{{Start: 4, End: 7}, {Start: 2, End: 2}, {Start: 0, End: 2}}
	doc.Editor.SetSelection(ranges[2])
	state.multiSelections[0] = multiSelectionState{doc: doc, value: doc.Editor.String(), active: true, ranges: ranges}
	return state
}
